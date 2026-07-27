package middleware

import (
	"github.com/gofiber/fiber/v2"
)

func ExtractAgentID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		agentID := c.Get("X-Agent-ID")
		c.Locals("agentID", agentID) // Pass to downstream

		return c.Next()
	}
}
