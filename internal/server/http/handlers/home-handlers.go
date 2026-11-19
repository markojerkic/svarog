package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type homeProjects interface {
	assignedProjects
	All(context.Context) ([]projects.Project, error)
}

type HomeHandler struct {
	group           *echo.Group
	projectsService homeProjects
}

func NewHomeHandler(group *echo.Group, projectsService homeProjects) *HomeHandler {
	handler := &HomeHandler{group, projectsService}
	handler.registerRoutes()
	return handler
}

func (h *HomeHandler) homeHandler(c *echo.Context) error {
	user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
	if !ok || user == nil {
		return htmx.Redirect(c, "/login")
	}
	var visibleProjects []projects.Project
	var err error
	if user.Role == auth.ADMIN {
		visibleProjects, err = h.projectsService.All(c.Request().Context())
	} else if len(user.ProjectIDs) > 0 {
		visibleProjects, err = h.projectsService.ByIDs(c.Request().Context(), user.ProjectIDs)
	}
	if err != nil {
		return err
	}

	props := pages.HomepageProps{Projects: visibleProjects}
	return utils.Render(c, http.StatusOK, pages.Homepage(props))
}

func (h *HomeHandler) registerRoutes() {
	h.group.GET("/", h.homeHandler)
}
