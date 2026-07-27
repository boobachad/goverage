package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	_ "github.com/lib/pq"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"

	"goverage/gateway/audit"
	"goverage/gateway/middleware"
	"goverage/gateway/routes"
)

type GatewayPolicyRule struct {
	Name                 string `yaml:"name"`
	Description          string `yaml:"description"`
	agentmesh.PolicyRule `yaml:",inline"`
}

type DynamicPolicyEngine struct {
	mu     sync.RWMutex
	rules  []GatewayPolicyRule
	engine *agentmesh.PolicyEngine
}

var (
	BroadcastHub = make(chan interface{}, 100)
	lifecycleMap sync.Map
)
var clients = make(map[*websocket.Conn]bool)
var clientsMu sync.Mutex

func broadcastMessages() {
	for msg := range BroadcastHub {
		clientsMu.Lock()
		for client := range clients {
			if err := client.WriteJSON(msg); err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		clientsMu.Unlock()
	}
}

func (d *DynamicPolicyEngine) EvaluateDetailed(action string, ctx map[string]interface{}) (agentmesh.PolicyDecision, string, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.engine == nil {
		return agentmesh.Deny, "default", "No policy engine loaded"
	}

	for _, rule := range d.rules {
		testRule := rule.PolicyRule
		testRule.Effect = agentmesh.Allow
		tmpEngine := agentmesh.NewPolicyEngine([]agentmesh.PolicyRule{testRule})
		if tmpEngine.Evaluate(action, ctx) == agentmesh.Allow {
			return rule.PolicyRule.Effect, rule.Name, rule.Description
		}
	}

	return agentmesh.Deny, "default", "No matching policy found"
}

func (d *DynamicPolicyEngine) Load(yamlData []byte) error {
	var config struct {
		Rules []GatewayPolicyRule `yaml:"rules"`
	}
	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return err
	}
	var agentRules []agentmesh.PolicyRule
	for _, r := range config.Rules {
		agentRules = append(agentRules, r.PolicyRule)
	}
	newEngine := agentmesh.NewPolicyEngine(agentRules)
	d.mu.Lock()
	d.rules = config.Rules
	d.engine = newEngine
	d.mu.Unlock()
	return nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment")
	}

	go broadcastMessages()

	// Initialize Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("FATAL: REDIS_ADDR environment variable is missing")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Initialize Postgres
	pgConn := os.Getenv("PG_CONN")
	if pgConn == "" {
		log.Fatal("FATAL: PG_CONN environment variable is missing")
	}
	db, err := sql.Open("postgres", pgConn)
	if err != nil {
		log.Fatal("Failed to connect to postgres:", err)
	}
	defer db.Close()

	// Start Async Audit logger
	audit.StartWorker(db)

	// Initialize Halt Registry (AgentMesh SDK)
	haltRegistry := agentmesh.NewKillSwitchRegistry()

	// Initialize Policy Engine (ABAC)
	dynEngine := &DynamicPolicyEngine{}
	if yamlData, err := os.ReadFile("policy.yaml"); err == nil {
		if err := dynEngine.Load(yamlData); err != nil {
			log.Println("Error loading policy.yaml on startup:", err)
		}
	} else {
		log.Println("policy.yaml not found on startup")
	}

	// Initialize Fiber app
	app := fiber.New()
	app.Use(cors.New())
	app.Use(logger.New())

	// Policy POST endpoint
	app.Post("/api/policy", func(c *fiber.Ctx) error {
		body := c.Body()
		if err := os.WriteFile("policy.yaml", body, 0644); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to write policy.yaml"})
		}
		if err := dynEngine.Load(body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "failed to parse or load policy YAML"})
		}
		return c.JSON(fiber.Map{"status": "policy hot-reloaded"})
	})

	app.Get("/api/policy", func(c *fiber.Ctx) error {
		yamlData, err := os.ReadFile("policy.yaml")
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "policy not found"})
		}
		return c.SendString(string(yamlData))
	})

	app.Get("/api/agents/status", func(c *fiber.Ctx) error {
		dids := globalRegistry.GetAllDIDs()
		var results []map[string]interface{}

		for _, did := range dids {
			ctx := context.Background()

			// Get remaining budget from Redis
			cap, _ := rdb.Get(ctx, "budget:agent:"+did).Int()

			var used float64
			err := db.QueryRow("SELECT COALESCE(SUM(cost), 0)::float FROM audit_logs WHERE agent_id = $1", did).Scan(&used)
			if err != nil {
				used = 0
			}

			// Check KillSwitch status
			decision := haltRegistry.DecisionFor(did, "")
			status := "active"
			if globalRegistry.IsTerminated(did) {
				status = "terminated"
			} else if !decision.Allowed {
				status = "halted"
			} else {
				if val, ok := lifecycleMap.Load(did); ok {
					manager := val.(*agentmesh.LifecycleManager)
					status = string(manager.State())
				}
			}

			results = append(results, map[string]interface{}{
				"did":       did,
				"cap":       cap,
				"used":      int(used),
				"remaining": cap - int(used),
				"status":    status,
			})
		}
		if results == nil {
			results = make([]map[string]interface{}, 0)
		}

		ctx := context.Background()
		fleetBudget, _ := rdb.Get(ctx, "budget:fleet").Int()

		return c.JSON(fiber.Map{
			"fleet_budget": fleetBudget,
			"agents":       results,
		})
	})

	// Two-Phase Token Budgets POST endpoint
	app.Post("/api/budget", func(c *fiber.Ctx) error {
		var req struct {
			Target string `json:"target"`
			Amount int    `json:"amount"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		ctx := context.Background()
		if req.Target == "fleet" {
			rdb.Set(ctx, "budget:fleet", req.Amount, 0)
		} else {
			rdb.Set(ctx, "budget:agent:"+req.Target, req.Amount, 0)
		}
		return c.JSON(fiber.Map{"status": "budget set"})
	})

	app.Get("/api/audit", func(c *fiber.Ctx) error {
		rows, err := db.Query("SELECT json_build_object('id', id, 'agent_id', agent_id, 'cost', cost, 'status', status, 'timestamp', timestamp, 'request_payload', request_payload, 'response_payload', response_payload) FROM audit_logs ORDER BY id DESC LIMIT 1000")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		var logs []interface{}
		for rows.Next() {
			var logJSON []byte
			if err := rows.Scan(&logJSON); err != nil {
				continue
			}
			var logObj interface{}
			if err := json.Unmarshal(logJSON, &logObj); err == nil {
				logs = append(logs, logObj)
			}
		}
		if logs == nil {
			logs = []interface{}{}
		}
		return c.JSON(logs)
	})

	// Identity & Verification
	app.Post("/api/register", RegisterAgent)
	app.Post("/api/unregister", UnregisterAgent)

	// Apply ABAC and execution halt middleware
	app.Use("/v1/chat/completions", IdentityMiddleware())
	app.Use("/v1/chat/completions", middleware.ExtractAgentID())
	app.Use("/v1/chat/completions", middleware.ExecutionHalt(haltRegistry, nil, BroadcastHub))

	// INIT Lease Manager
	chunkStr := os.Getenv("LEASE_CHUNK_SIZE")
	if chunkStr == "" {
		log.Fatal("FATAL: LEASE_CHUNK_SIZE environment variable is missing")
	}
	chunkSize, err := strconv.ParseInt(chunkStr, 10, 64)
	if err != nil {
		log.Fatal("FATAL: LEASE_CHUNK_SIZE must be an integer")
	}

	refillStr := os.Getenv("LEASE_REFILL_THRESHOLD")
	if refillStr == "" {
		log.Fatal("FATAL: LEASE_REFILL_THRESHOLD environment variable is missing")
	}
	refillThreshold, err := strconv.ParseInt(refillStr, 10, 64)
	if err != nil {
		log.Fatal("FATAL: LEASE_REFILL_THRESHOLD must be an integer")
	}

	leaseManager := routes.NewLeaseManager(rdb, chunkSize, refillThreshold)

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("FATAL: OPENROUTER_API_KEY environment variable is missing. The Gateway requires the master API key to enforce centralized budget controls.")
	}

	auditLogger := agentmesh.NewAuditLogger()

	// Routes
	app.Get("/api/system/audit/verify", func(c *fiber.Ctx) error {
		intact := auditLogger.Verify()
		entries := auditLogger.GetEntries(agentmesh.AuditFilter{})
		return c.JSON(fiber.Map{
			"intact":  intact,
			"entries": entries,
		})
	})

	app.Post("/api/lifecycle/suspend", func(c *fiber.Ctx) error {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		if val, ok := lifecycleMap.Load(req.AgentID); ok {
			manager := val.(*agentmesh.LifecycleManager)
			if _, err := manager.Suspend("Operator manual suspension"); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"status": "suspended", "agent_id": req.AgentID})
		}
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	})

	app.Post("/api/lifecycle/quarantine", func(c *fiber.Ctx) error {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		if val, ok := lifecycleMap.Load(req.AgentID); ok {
			manager := val.(*agentmesh.LifecycleManager)
			if _, err := manager.Quarantine("Operator manual quarantine"); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"status": "quarantined", "agent_id": req.AgentID})
		}
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	})

	app.Post("/api/lifecycle/resume", func(c *fiber.Ctx) error {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		if val, ok := lifecycleMap.Load(req.AgentID); ok {
			manager := val.(*agentmesh.LifecycleManager)
			if _, err := manager.Activate("Operator manual resume"); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"status": "active", "agent_id": req.AgentID})
		}
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	})

	routes.SetupProxy(app, rdb, dynEngine, leaseManager, apiKey, BroadcastHub, auditLogger, &lifecycleMap)

	// Ensure WebSocket upgrade
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/telemetry", websocket.New(func(c *websocket.Conn) {
		clientsMu.Lock()
		clients[c] = true
		clientsMu.Unlock()

		defer func() {
			clientsMu.Lock()
			delete(clients, c)
			clientsMu.Unlock()
			c.Close()
		}()

		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	app.Post("/api/halt", func(c *fiber.Ctx) error {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		var scopeObj agentmesh.KillSwitchScope
		if req.AgentID == "fleet" || req.AgentID == "" {
			scopeObj = agentmesh.GlobalKillSwitchScope()
		} else {
			scopeObj = agentmesh.AgentKillSwitchScope(req.AgentID)
		}
		_, err := haltRegistry.Activate(scopeObj, agentmesh.KillSwitchReasonOperatorRequest, "Operator requested halt")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "halted", "agent_id": req.AgentID})
	})

	app.Post("/api/resume", func(c *fiber.Ctx) error {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		var scopeObj agentmesh.KillSwitchScope
		if req.AgentID == "fleet" || req.AgentID == "" {
			scopeObj = agentmesh.GlobalKillSwitchScope()
		} else {
			scopeObj = agentmesh.AgentKillSwitchScope(req.AgentID)
		}
		_, err := haltRegistry.Clear(scopeObj, agentmesh.KillSwitchReasonOperatorRequest, "Operator requested resume")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "resumed", "agent_id": req.AgentID})
	})

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("FATAL: PORT environment variable is missing")
	}
	log.Println("Starting Gateway on port", port)
	log.Fatal(app.Listen(":" + port))
}
