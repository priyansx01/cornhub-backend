package course

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/priyansx01/corn-hub-clone/internal/domain"
	"github.com/priyansx01/corn-hub-clone/internal/middleware"
	"github.com/priyansx01/corn-hub-clone/pkg/response"
)

// Handler exposes course HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new course handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts course routes.
func (h *Handler) RegisterRoutes(api fiber.Router) {
	// Only instructors and admins may create or change content.
	staff := middleware.RequireRole(string(domain.RoleInstructor), string(domain.RoleAdmin))

	api.Get("/categories", h.ListCategories)

	courses := api.Group("/courses")
	courses.Get("/", h.List)
	courses.Get("/:id", h.Get)
	courses.Post("/", staff, h.Create)
	courses.Patch("/:id", staff, h.Update)
	courses.Delete("/:id", staff, h.Delete)

	// Modules
	courses.Get("/:id/modules", h.ListModules)
	courses.Post("/:id/modules", staff, h.CreateModule)
	courses.Delete("/:id/modules/:moduleId", staff, h.DeleteModule)

	// Upload + Playback
	courses.Post("/:id/modules/:moduleId/upload", staff, h.UploadFile)
	courses.Post("/:id/modules/:moduleId/upload-url", staff, h.GetUploadURL)
	courses.Post("/:id/modules/:moduleId/upload-complete", staff, h.CompleteUpload)
	courses.Get("/:id/modules/:moduleId/playback-url", h.GetPlaybackURL)
	courses.Get("/:id/modules/:moduleId/progress", h.GetModuleProgress)
}

// visibleStatus returns the status filter to apply for the caller. Learners
// only ever see ready courses (architecture §4.4); staff see everything they
// ask for.
func visibleStatus(c *fiber.Ctx) string {
	if role, _ := c.Locals("role").(string); role == string(domain.RoleEmployee) {
		return string(domain.CourseStatusReady)
	}
	return c.Query("status")
}

// ─── Course Handlers ──────────────────────────────────────────────────────────

func (h *Handler) List(c *fiber.Ctx) error {
	courses, err := h.svc.ListCourses(
		visibleStatus(c),
		c.Query("search"),
		c.Query("category"),
	)
	if err != nil {
		return response.InternalError(c, "Failed to list courses")
	}
	if courses == nil {
		courses = []domain.Course{} // never return null
	}
	return response.OK(c, fiber.Map{
		"items":       courses,
		"total":       len(courses),
		"page":        1,
		"per_page":    len(courses),
		"total_pages": 1,
	})
}

func (h *Handler) ListCategories(c *fiber.Ctx) error {
	cats, err := h.svc.ListCategories(visibleStatus(c))
	if err != nil {
		return response.InternalError(c, "Failed to list categories")
	}
	if cats == nil {
		cats = []CategoryCount{}
	}
	return response.OK(c, cats)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	course, err := h.svc.GetCourse(c.Params("id"))
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalError(c, "Failed to get course")
	}
	if want := visibleStatus(c); want == string(domain.CourseStatusReady) && course.Status != domain.CourseStatusReady {
		return response.NotFound(c, "Course not found")
	}
	return response.OK(c, course)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Title == "" {
		return response.BadRequest(c, "Title is required")
	}

	userID := c.Locals("user_id").(string)
	course, err := h.svc.CreateCourse(userID, req)
	if err != nil {
		return response.InternalError(c, "Failed to create course: "+err.Error())
	}
	return response.Created(c, course)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	var req CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	course, err := h.svc.UpdateCourse(c.Params("id"), req)
	if err != nil {
		return response.InternalError(c, "Failed to update course")
	}
	return response.OK(c, course)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	if err := h.svc.DeleteCourse(c.Params("id")); err != nil {
		return response.InternalError(c, "Failed to delete course")
	}
	return response.NoContent(c)
}

// ─── Module Handlers ──────────────────────────────────────────────────────────

func (h *Handler) ListModules(c *fiber.Ctx) error {
	modules, err := h.svc.ListModules(c.Params("id"))
	if err != nil {
		return response.InternalError(c, "Failed to list modules")
	}
	if modules == nil {
		modules = []domain.Module{}
	}
	return response.OK(c, modules)
}

func (h *Handler) CreateModule(c *fiber.Ctx) error {
	var req CreateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	module, err := h.svc.CreateModule(c.Params("id"), req)
	if err != nil {
		return response.InternalError(c, "Failed to create module")
	}
	return response.Created(c, module)
}

func (h *Handler) DeleteModule(c *fiber.Ctx) error {
	if err := h.svc.DeleteModule(c.Params("id"), c.Params("moduleId")); err != nil {
		return response.InternalError(c, "Failed to delete module")
	}
	return response.NoContent(c)
}

// ─── Upload + Playback ────────────────────────────────────────────────────────

func (h *Handler) UploadFile(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "Missing file in request")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return response.InternalError(c, "Failed to open uploaded file")
	}
	defer file.Close()

	userID := c.Locals("user_id").(string)
	err = h.svc.UploadModuleFile(c.Context(), userID, c.Params("id"), c.Params("moduleId"), fileHeader.Filename, file, fileHeader.Size, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		log.Printf("❌ File upload failed: %v", err)
		return response.InternalError(c, "Failed to upload file and trigger processing")
	}

	return response.OK(c, fiber.Map{"message": "Upload complete, processing started"})
}

func (h *Handler) GetUploadURL(c *fiber.Ctx) error {
	var req UploadURLRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	url, objectKey, err := h.svc.GetUploadURL(c.Params("id"), c.Params("moduleId"), req.FileName)
	if err != nil {
		return response.InternalError(c, "Failed to generate upload URL")
	}

	return response.OK(c, fiber.Map{
		"upload_url": url,
		"object_key": objectKey,
		"expires_at": "1h",
	})
}

func (h *Handler) CompleteUpload(c *fiber.Ctx) error {
	var req UploadURLRequest // reusing the struct for file_name
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userID := c.Locals("user_id").(string)
	err := h.svc.CompleteUpload(c.Context(), userID, c.Params("id"), c.Params("moduleId"), req.FileName)
	if err != nil {
		return response.InternalError(c, "Failed to complete upload and trigger processing")
	}

	return response.OK(c, fiber.Map{"message": "Upload complete, processing started"})
}

func (h *Handler) GetPlaybackURL(c *fiber.Ctx) error {
	url, err := h.svc.GetPlaybackURL(c.Params("id"), c.Params("moduleId"))
	if err != nil {
		switch {
		case errors.Is(err, ErrModuleNotFound):
			return response.NotFound(c, "Module not found")
		case errors.Is(err, ErrModuleNotReady):
			return response.Error(c, fiber.StatusConflict, "Video is still processing")
		}
		return response.InternalError(c, "Failed to generate playback URL")
	}

	return response.OK(c, fiber.Map{
		"playback_url": url,
		"type":         "application/vnd.apple.mpegurl",
	})
}

func (h *Handler) GetModuleProgress(c *fiber.Ctx) error {
	progress, err := h.svc.GetModuleProgress(c.Context(), c.Params("moduleId"))
	if err != nil {
		return response.InternalError(c, "Failed to fetch module progress")
	}
	return response.OK(c, progress)
}
