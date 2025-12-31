package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type HomeHandler struct {
	group           *echo.Group
	projectsService projects.ProjectsService
}

func NewHomeHandler(group *echo.Group, projectsService projects.ProjectsService) *HomeHandler {
	handler := &HomeHandler{group, projectsService}
	handler.registerRoutes()
	return handler
}

func (h *HomeHandler) homeHandler(c echo.Context) error {
	projects, err := h.projectsService.GetProjects(c.Request().Context())
	if err != nil {
		return err
	}

	props := pages.HomepageProps{Projects: projects}
	return utils.Render(c, http.StatusOK, pages.Homepage(props))
}

func (h *HomeHandler) registerRoutes() {
	h.group.GET("/", h.homeHandler)
}
