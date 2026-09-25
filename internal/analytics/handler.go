package analytics

import (
	"github.com/gofiber/fiber/v2"
	"github.com/priyansx01/corn-hub-clone/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(api fiber.Router) {
	analytics := api.Group("/analytics")
	analytics.Post("/events/video-watched", h.TrackVideoWatched)
	analytics.Post("/events/quiz-attempted", h.TrackQuizAttempted)
	analytics.Post("/events/drop-off", h.TrackDropOff)
	
	// Example dashboard metric
	analytics.Get("/modules/:id/avg-watch", h.GetModuleAvgWatch)
	
	// Dashboard Aggregations
	analytics.Get("/dashboard/overview", h.GetDashboardOverview)
	analytics.Get("/dashboard/dropoff", h.GetDashboardDropoff)
}

func (h *Handler) TrackVideoWatched(c *fiber.Ctx) error {
	var req struct {
		CourseID string  `json:"course_id"`
		ModuleID string  `json:"module_id"`
		WatchPct float32 `json:"watch_pct"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userID := c.Locals("user_id").(string)
	if err := h.svc.TrackVideoWatched(c.Context(), userID, req.CourseID, req.ModuleID, req.WatchPct); err != nil {
		return response.InternalError(c, "Failed to track event")
	}

	return response.OK(c, fiber.Map{"status": "recorded"})
}

func (h *Handler) TrackQuizAttempted(c *fiber.Ctx) error {
	var req struct {
		CourseID string `json:"course_id"`
		Score    int    `json:"score"`
		Passed   bool   `json:"passed"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userID := c.Locals("user_id").(string)
	if err := h.svc.TrackQuizAttempted(c.Context(), userID, req.CourseID, req.Score, req.Passed); err != nil {
		return response.InternalError(c, "Failed to track event")
	}

	return response.OK(c, fiber.Map{"status": "recorded"})
}

func (h *Handler) TrackDropOff(c *fiber.Ctx) error {
	var req struct {
		ModuleID       string `json:"module_id"`
		SecondsWatched int    `json:"seconds_watched"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userID := c.Locals("user_id").(string)
	if err := h.svc.TrackDropOff(c.Context(), userID, req.ModuleID, req.SecondsWatched); err != nil {
		return response.InternalError(c, "Failed to track event")
	}

	return response.OK(c, fiber.Map{"status": "recorded"})
}

func (h *Handler) GetModuleAvgWatch(c *fiber.Ctx) error {
	avg, err := h.svc.GetModuleAvgWatchPct(c.Context(), c.Params("id"))
	if err != nil {
		return response.InternalError(c, "Failed to fetch metrics")
	}
	return response.OK(c, fiber.Map{"module_id": c.Params("id"), "avg_watch_pct": avg})
}

func (h *Handler) GetDashboardOverview(c *fiber.Ctx) error {
	metrics, err := h.svc.GetOverviewMetrics(c.Context())
	if err != nil {
		return response.InternalError(c, "Failed to fetch dashboard overview")
	}
	return response.OK(c, metrics)
}

func (h *Handler) GetDashboardDropoff(c *fiber.Ctx) error {
	data, err := h.svc.GetVideoDropoffData(c.Context())
	if err != nil {
		return response.InternalError(c, "Failed to fetch dropoff data")
	}
	return response.OK(c, data)
}
