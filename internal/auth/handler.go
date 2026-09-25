package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/priyansx01/corn-hub-clone/pkg/response"
)

// Handler exposes auth HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new auth handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts auth routes on the given router group.
func (h *Handler) RegisterRoutes(api fiber.Router) {
	auth := api.Group("/auth")
	auth.Post("/login", h.Login)
	auth.Post("/refresh", h.Refresh)
	auth.Get("/me", h.Me) // protected by middleware at the router level
}

// Login handles POST /auth/login
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	resp, err := h.svc.Login(req)
	if err != nil {
		return response.Unauthorized(c, "Invalid email or password")
	}

	return response.OK(c, resp)
}

// Refresh handles POST /auth/refresh
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	accessToken, err := h.svc.Refresh(req)
	if err != nil {
		return response.Unauthorized(c, "Invalid or expired refresh token")
	}

	return response.OK(c, fiber.Map{"access_token": accessToken})
}

// Me handles GET /auth/me — returns the current user profile.
func (h *Handler) Me(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	user, err := h.svc.GetUserByID(userID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.OK(c, user)
}
