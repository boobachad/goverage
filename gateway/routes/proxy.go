package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"goverage/gateway/audit"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
	"github.com/redis/go-redis/v9"
)

type PolicyEvaluator interface {
	EvaluateDetailed(action string, context map[string]interface{}) (agentmesh.PolicyDecision, string, string)
}

func SetupProxy(
	app *fiber.App,
	rdb *redis.Client,
	engine PolicyEvaluator,
	lm *LeaseManager,
	apiKey string,
	broadcastHub chan interface{},
	auditLogger *agentmesh.AuditLogger,
	lifecycleMap *sync.Map,
) {
	upstreamAPI := os.Getenv("UPSTREAM_API")
	if upstreamAPI == "" {
		log.Fatal("FATAL: UPSTREAM_API environment variable is missing")
	}

	app.Post("/v1/chat/completions", func(c *fiber.Ctx) error {
		agentIDRaw := c.Locals("agentID")
		agentIDUnsafe, ok := agentIDRaw.(string)
		if !ok || agentIDUnsafe == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing agent ID in context"})
		}
		// must force-allocate a new backing array to detach it from the fasthttp worker pool.
		agentID := string(append([]byte(nil), agentIDUnsafe...))

		var manager *agentmesh.LifecycleManager
		if val, ok := lifecycleMap.Load(agentID); ok {
			manager = val.(*agentmesh.LifecycleManager)
		} else {
			manager = agentmesh.NewLifecycleManager(agentID)
			manager.Activate("Auto-activated on first proxy request")
			lifecycleMap.Store(agentID, manager)
		}

		if manager.State() != agentmesh.StateActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": fmt.Sprintf("invalid transition: agent state is %s", manager.State())})
		}

		var reqPayload interface{}
		if err := json.Unmarshal(c.Body(), &reqPayload); err != nil {
			reqPayload = string(c.Body())
		}

		cost := 512
		if reqMap, ok := reqPayload.(map[string]interface{}); ok {
			if maxTokens, ok := reqMap["max_tokens"].(float64); ok {
				cost = int(maxTokens)
			}
		}

		// Reserve Budget
		if !lm.Consume(int64(cost)) {
			// Trigger SDK Lifecycle Transition: Suspend agent when budget exhausts
			manager.Suspend("Token budget exhausted")

			audit.LogCh <- audit.LogEntry{
				AgentID:         agentID,
				RequestPayload:  reqPayload,
				ResponsePayload: nil,
				Cost:            0,
				Status:          "budget_exhausted",
			}
			if broadcastHub != nil {
				broadcastHub <- map[string]interface{}{
					"level":     "error",
					"message":   "Token budget exhausted - Agent suspended",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"agent_id":  agentID,
				}
			}
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Token budget exhausted"})
		}

		c.Request().Header.Set("Authorization", "Bearer "+apiKey)
		c.Request().Header.Del("Accept-Encoding")

		if err := proxy.Do(c, upstreamAPI); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "upstream failed"})
		}

		// Intercept response
		respBytes := c.Response().Body()
		var respBody map[string]interface{}
		actualCost := cost
		mutated := false
		if err := json.Unmarshal(respBytes, &respBody); err == nil {
			// Extract actual tokens
			var totalTokens int
			if usage, ok := respBody["usage"].(map[string]interface{}); ok {
				if tt, ok := usage["total_tokens"].(float64); ok {
					totalTokens = int(tt)
					actualCost = totalTokens
				}
			}

			// Refund remaining
			refundAmt := cost - actualCost
			if refundAmt > 0 {
				lm.Refund(int64(refundAmt))
			}

			// Parse tool calls and enforce policy
			if choices, ok := respBody["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if msg, ok := choice["message"].(map[string]interface{}); ok {
						if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
							for _, tc := range toolCalls {
								if tcm, ok := tc.(map[string]interface{}); ok {
									if fn, ok := tcm["function"].(map[string]interface{}); ok {
										if name, ok := fn["name"].(string); ok {
											decision, ruleName, description := engine.EvaluateDetailed(name, map[string]interface{}{"agent_id": agentID})

											if auditLogger != nil {
												auditLogger.Log(agentID, name, agentmesh.PolicyDecision(decision))
											}

											if decision != agentmesh.Allow {
												msg["tool_calls"] = nil
												delete(msg, "tool_calls")
												errMsg := fmt.Sprintf("GovernanceDenied: Action denied by policy rule '%s': %s", ruleName, description)
												msg["content"] = errMsg
												mutated = true
												if broadcastHub != nil {
													broadcastHub <- map[string]interface{}{
														"level":     "error",
														"message":   errMsg,
														"timestamp": time.Now().UTC().Format(time.RFC3339),
														"agent_id":  agentID,
													}
												}
												break
											}
										}
									}
								}
							}
						}
					}
				}
			}

			if mutated {
				newBody, _ := json.Marshal(respBody)
				c.Response().SetBody(newBody)
				respBody["content"] = string(newBody) // just for audit log
			} else {
				c.Response().Header.Del("Transfer-Encoding")
			}
		}

		// Log asyncly to Postgres
		var finalResp interface{}
		if err := json.Unmarshal(c.Response().Body(), &finalResp); err != nil {
			finalResp = map[string]interface{}{
				"raw_response": string(c.Response().Body()),
				"error":        err.Error(),
			}
		}

		audit.LogCh <- audit.LogEntry{
			AgentID:         agentID,
			RequestPayload:  reqPayload,
			ResponsePayload: finalResp,
			Cost:            float64(actualCost),
			Status:          "success",
		}

		if auditLogger != nil {
			decision := agentmesh.Allow
			if mutated {
				decision = agentmesh.Deny
			}
			auditLogger.Log(agentID, "/v1/chat/completions", decision)
		}

		if broadcastHub != nil && !mutated {
			broadcastHub <- map[string]interface{}{
				"level":     "info",
				"message":   fmt.Sprintf("Request routed successfully (Cost: %d tokens)", actualCost),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"agent_id":  agentID,
			}
		}

		return nil
	})
}
