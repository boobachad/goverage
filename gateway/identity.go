package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"sync"
	"github.com/gofiber/fiber/v2"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
)

type AgentRegistry struct {
	mu         sync.RWMutex
	store      map[string]string // DID -> base64 public key
	terminated map[string]bool   // DID -> true if terminated
}

var globalRegistry = &AgentRegistry{
	store:      make(map[string]string),
	terminated: make(map[string]bool),
}

func (r *AgentRegistry) IsTerminated(did string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.terminated[did]
}

func (r *AgentRegistry) GetAllDIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dids := make([]string, 0, len(r.store))
	for did := range r.store {
		dids = append(dids, did)
	}
	return dids
}

func RegisterAgent(c *fiber.Ctx) error {
	var req struct {
		DID       string `json:"did"`
		PublicKey string `json:"public_key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	globalRegistry.mu.Lock()
	globalRegistry.store[req.DID] = req.PublicKey
	delete(globalRegistry.terminated, req.DID)
	globalRegistry.mu.Unlock()

	// Initialize the LifecycleManager for this agent
	manager := agentmesh.NewLifecycleManager(req.DID)
	manager.Activate("initial registration")
	lifecycleMap.Store(req.DID, manager)

	return c.SendStatus(fiber.StatusOK)
}

func UnregisterAgent(c *fiber.Ctx) error {
	var req struct {
		DID string `json:"did"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	globalRegistry.mu.Lock()
	globalRegistry.terminated[req.DID] = true
	globalRegistry.mu.Unlock()

	return c.SendStatus(fiber.StatusOK)
}

func IdentityMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		did := c.Get("X-Agent-DID")
		pubKeyB64 := c.Get("X-Agent-Public-Key")
		sigB64 := c.Get("X-Agent-Signature")

		if did == "" || pubKeyB64 == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing identity headers"})
		}

		globalRegistry.mu.RLock()
		registeredKey, exists := globalRegistry.store[did]
		globalRegistry.mu.RUnlock()

		if !exists || registeredKey != pubKeyB64 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "identity verification failed: key mismatch"})
		}

		// Mathematical verification of Ed25519 signature
		pubKey, err := base64.StdEncoding.DecodeString(pubKeyB64)
		if err != nil || len(pubKey) != ed25519.PublicKeySize {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid public key format"})
		}

		if sigB64 == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing signature"})
		}

		sig, err := base64.StdEncoding.DecodeString(sigB64)
		if err != nil || len(sig) != ed25519.SignatureSize {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature format"})
		}
		
		// Verify over the body
		if !ed25519.Verify(pubKey, c.Body(), sig) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
		}

		return c.Next()
	}
}
