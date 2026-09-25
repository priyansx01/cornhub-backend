// Package middleware provides Fiber middleware for the API gateway.
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/priyansx01/corn-hub-clone/internal/auth"
	"github.com/priyansx01/corn-hub-clone/pkg/response"
)

// JWTAuth returns a Fiber middleware that validates the Authorization header
// and injects user_id + role into c.Locals for downstream handlers.
func JWTAuth(jwtMgr *auth.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return response.Unauthorized(c, "Missing authorization header")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		claims, err := jwtMgr.ValidateAccessToken(parts[1])
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired token")
		}

		// Inject into context for downstream handlers
		c.Locals("user_id", claims.LMSUserID)
		c.Locals("role", string(claims.Role))

		return c.Next()
	}
}

// RequireRole returns a middleware that restricts access to specific roles.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}

	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok || !allowed[role] {
			return response.Forbidden(c, "Insufficient permissions")
		}
		return c.Next()
	}
}
