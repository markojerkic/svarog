package handlers

import (
	"fmt"
	"net/http"

	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type ProjectsRouter struct {
	projectsService  projects.ProjectsService
	natsCredsService serverauth.NatsCredentialService
}

func (p *ProjectsRouter) getProjects(c echo.Context) error {
	projects, err := p.projectsService.GetProjects(c.Request().Context())
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		return c.JSON(500, types.ApiError{Message: "Error getting projects"})
	}

	return utils.Render(c, http.StatusOK, admin.ProjectsListPage(admin.ProjectsListPageProps{
		Projects: projects,
	}))
}

func (p *ProjectsRouter) getProject(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required", Fields: map[string]string{"id": "Project ID is required"}})
	}
	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error getting project"})
	}

	return c.JSON(200, project)
}

func (p *ProjectsRouter) getEditProjectForm(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required"})
	}

	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error getting project"})
	}

	return utils.Render(c, http.StatusOK, admin.NewProjectForm(admin.NewProjectFormProps{
		FormID: "edit-project-form",
		Value: types.CreateProjectForm{
			ID:      project.ID.Hex(),
			Name:    project.Name,
			Clients: project.Clients,
		},
	}))
}

func (p *ProjectsRouter) getConnectionStringForm(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required"})
	}

	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error getting project"})
	}

	return utils.Render(c, http.StatusOK, admin.ConnectionStringForm(admin.ConnectionStringFormProps{
		ProjectID: project.ID.Hex(),
		Clients:   project.Clients,
	}))
}

func (p *ProjectsRouter) createProject(c echo.Context) error {
	var createProjectForm types.CreateProjectForm
	if err := c.Bind(&createProjectForm); err != nil {
		return c.JSON(400, err)
	}
	if err := c.Validate(&createProjectForm); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			htmx.Reswap(c, htmx.ReswapProps{
				Swap:   "outerHTML",
				Target: "this",
				Select: "form",
			})
			return utils.Render(c, http.StatusBadRequest, admin.NewProjectForm(admin.NewProjectFormProps{
				ApiError: apiErr,
				Value:    createProjectForm,
			}))
		}

		return err
	}

	project, err := p.projectsService.CreateOrUpdateProject(c.Request().Context(), createProjectForm)
	if err != nil {
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: "this",
			Select: "form",
		})
		if err.Error() == projects.ErrProjectExists {
			return utils.Render(c, http.StatusConflict, admin.NewProjectForm(admin.NewProjectFormProps{
				ApiError: types.ApiError{
					Message: "Project already exists",
					Fields:  map[string]string{"name": "Project with this name already exists"}},
				Value: createProjectForm,
			}))
		}

		slog.Error("Error creating project", "error", err)
		return utils.Render(c, http.StatusInternalServerError, admin.NewProjectForm(admin.NewProjectFormProps{
			ApiError: types.ApiError{
				Message: "Error creating project",
			},
			Value: createProjectForm,
		}))
	}

	htmx.CloseDialog(c)
	if createProjectForm.ID != "" {
		htmx.AddSuccessToast(c, "Project updated")
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: fmt.Sprintf("[data-project-id='%s']", createProjectForm.ID),
			Select: "tr",
		})
	} else {
		htmx.AddSuccessToast(c, "Project created")
	}

	return utils.Render(c, http.StatusOK, admin.ProjectsTableBody(admin.ProjectsListPageProps{
		Projects: []projects.Project{project},
	}))
}

func (p *ProjectsRouter) deleteProject(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "Project ID is required", Fields: map[string]string{"id": "Project ID is required"}})
	}
	err := p.projectsService.DeleteProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error deleting project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: "Project not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error deleting project"})
	}

	htmx.AddSuccessToast(c, "Project deleted")
	return c.HTML(200, "")
}

func (p *ProjectsRouter) createProjectConnString(c echo.Context) error {
	var request serverauth.CredentialGenerationRequest
	if err := c.Bind(&request); err != nil {
		htmx.AddErrorToast(c, "Failed to generate credentials")
		return c.JSON(400, err)
	}
	if err := c.Validate(&request); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			htmx.Reswap(c, htmx.ReswapProps{
				Swap:   "outerHTML",
				Target: "this",
				Select: "form",
			})

			project, projErr := p.projectsService.GetProject(c.Request().Context(), request.ProjectID)
			if projErr != nil {
				htmx.AddErrorToast(c, "Failed to generate credentials")
				return c.JSON(500, types.ApiError{Message: "Error getting project"})
			}

			return utils.Render(c, http.StatusBadRequest, admin.ConnectionStringForm(admin.ConnectionStringFormProps{
				ProjectID: request.ProjectID,
				Clients:   project.Clients,
				ApiError:  apiErr,
			}))
		}
		htmx.AddErrorToast(c, "Failed to generate credentials")
		return c.JSON(400, err)
	}

	creds, err := p.natsCredsService.GenerateConnString(c.Request().Context(), request)
	if err != nil {
		htmx.AddErrorToast(c, "Failed to generate credentials")
		return c.JSON(500, types.ApiError{Message: "Error generating credentials"})
	}

	connString := creds.GetConnString()
	slog.Debug("Generated connection string", "connString", connString)

	htmx.CloseDialog(c)
	htmx.AddSuccessToast(c, "Connection string generated and copied to clipboard")

	c.Response().Header().Set("HX-Trigger-After-Swap", fmt.Sprintf(`{"copyToClipboard":"%s"}`, connString))

	return c.String(200, connString)
}

func NewProjectsRouter(
	projectsService projects.ProjectsService,
	natsCredsService serverauth.NatsCredentialService,
	e *echo.Group,
) *ProjectsRouter {
	router := &ProjectsRouter{projectsService, natsCredsService}

	if router.projectsService == nil {
		panic("No projectsService")
	}

	group := e.Group("/projects")
	group.GET("", router.getProjects)
	group.GET("/:id", router.getProject)
	group.GET("/:id/edit", router.getEditProjectForm)
	group.GET("/:id/connection-string-form", router.getConnectionStringForm)
	group.POST("", router.createProject)
	group.POST("/conn-string", router.createProjectConnString)
	group.DELETE("/:id", router.deleteProject)

	return router
}
