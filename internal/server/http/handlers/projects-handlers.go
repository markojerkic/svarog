package handlers

import (
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/types"
)

type ProjectsRouter struct {
	projectsService projects.ProjectsService
}

func (p *ProjectsRouter) getProjects(c echo.Context) error {
	projects, err := p.projectsService.GetProjects(c.Request().Context())
	if err != nil {
		log.Error("Error fetching project", "error", err)
		return c.JSON(401, types.ApiError{Message: "Error getting projects"})
	}

	return c.JSON(200, projects)
}

func (p *ProjectsRouter) getProject(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required", Fields: map[string]string{"id": "Project ID is required"}})
	}
	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		log.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error getting project"})
	}

	return c.JSON(200, project)
}

func (p *ProjectsRouter) getProjectByClient(c echo.Context) error {
	client := c.Param("client")
	if client == "" {
		return c.JSON(400, types.ApiError{Message: "Client ID is required", Fields: map[string]string{"id": "Client ID is required"}})
	}
	project, err := p.projectsService.GetProjectByClient(c.Request().Context(), client)
	if err != nil {
		log.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		} else {
			return c.JSON(500, types.ApiError{Message: "Error getting project"})
		}
	}

	return c.JSON(200, project)
}

func (p *ProjectsRouter) createProject(c echo.Context) error {
	var createProjectForm types.CreateProjectForm
	if err := c.Bind(&createProjectForm); err != nil {
		return c.JSON(400, err)
	}
	if err := c.Validate(&createProjectForm); err != nil {
		return err
	}

	project, err := p.projectsService.CreateProject(c.Request().Context(), createProjectForm.Name, createProjectForm.Clients)
	if err != nil {
		log.Error("Error creating project", "error", err)
		return c.JSON(500, types.ApiError{Message: "Error creating project"})
	}

	return c.JSON(200, project)
}

func (p *ProjectsRouter) logout(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required", Fields: map[string]string{"id": "Project ID is required"}})
	}
	err := p.projectsService.DeleteProject(c.Request().Context(), id)
	if err != nil {
		log.Error("Error deleting project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error deleting project"})
	}
	return c.JSON(200, "Project deleted")
}

func (p *ProjectsRouter) removeClientFromProject(c echo.Context) error {
	var removeClientForm types.RemoveClientForm
	if err := c.Bind(&removeClientForm); err != nil {
		return c.JSON(400, err)
	}

	if err := c.Validate(&removeClientForm); err != nil {
		return err
	}

	err := p.projectsService.RemoveClientFromProject(c.Request().Context(), removeClientForm.ProjectID, removeClientForm.Client)
	if err != nil {
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error removing client project"})
	}

	return c.JSON(200, "Client removed from project")
}

func (p *ProjectsRouter) addClientToProject(c echo.Context) error {
	var addClientForm types.AddClientForm
	if err := c.Bind(&addClientForm); err != nil {
		return c.JSON(400, err)
	}

	if err := c.Validate(&addClientForm); err != nil {
		return err
	}

	err := p.projectsService.AddClientToProject(c.Request().Context(), addClientForm.ProjectId, addClientForm.ClientId)
	if err != nil {
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error adding client to project"})
	}

	return c.JSON(200, "Client added to project")
}

func NewProjectsRouter(projectsService projects.ProjectsService, e *echo.Group) *ProjectsRouter {
	router := &ProjectsRouter{projectsService}

	if router.projectsService == nil {
		panic("No projectsService")
	}

	group := e.Group("/projects", middleware.RequiresRoleMiddleware(auth.ADMIN))
	group.GET("", router.getProjects)
	group.GET("/:id", router.getProject)
	group.GET("/client/:client", router.getProjectByClient)
	group.POST("", router.createProject)
	group.POST("/remove-client", router.removeClientFromProject)
	group.POST("/add-client", router.addClientToProject)
	group.DELETE("/:id", router.logout)

	return router
}
