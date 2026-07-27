package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
)

// ExecutionHalt applies tier-based halting and blocks requests if an execution halt is active or trust tier is low.
func ExecutionHalt(haltRegistry *agentmesh.KillSwitchRegistry, tm *agentmesh.TrustManager, broadcastHub chan interface{}) fiber.Handler {
	return func(c *fiber.Ctx) error {
		agentID := c.Get("X-Agent-DID")
		if agentID == "" {
			agentID = c.Get("X-Agent-ID")
		}
		if agentID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "X-Agent-ID or X-Agent-DID header required"})
		}

		// Tier-based halting
		if tm != nil {
			score := tm.GetTrustScore(agentID)
			if score.Tier == "low" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error":  "Agent execution halted",
					"reason": "trust_tier_low",
				})
			}
		}

		// Check halt decision for this agent
		// Note: agentmesh.KillSwitchRegistry has a DecisionFor method
		if haltRegistry != nil {
			decision := haltRegistry.DecisionFor(agentID, "")
			if !decision.Allowed {
				if broadcastHub != nil {
					msg := "GovernanceDenied: execution halted"
					if decision.Scope != nil {
						msg = "GovernanceDenied: execution halted: " + decision.Scope.String()
					}
					broadcastHub <- map[string]interface{}{
						"level":     "error",
						"message":   msg,
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"agent_id":  agentID,
						"reason":    decision.Event.Reason,
					}
				}
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error":  "Agent execution halted",
					"reason": decision.Event.Reason,
				})
			}
		}

		// Pass context for further handlers if needed
		c.Locals("agentID", agentID)

		return c.Next()
	}
}
