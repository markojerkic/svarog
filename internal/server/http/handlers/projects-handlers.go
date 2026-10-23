package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"log/slog"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/combobox"
	projectcomponents "github.com/markojerkic/svarog/internal/server/ui/components/projects"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type ProjectsRouter struct {
	projectsService  projects.ProjectsService
	natsCredsService serverauth.NatsCredentialService
}

func (p *ProjectsRouter) searchProjects(c *echo.Context) error {
	var request projects.SearchProjectsRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, types.ApiError{Message: i18n.T(c.Request().Context(), "request.invalid_parameters")})
	}

	if request.Size == 0 {
		request.Size = 10
	}

	if err := utils.Validate(c, &request); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			return c.JSON(http.StatusBadRequest, apiErr)
		}
		return c.JSON(http.StatusBadRequest, types.ApiError{Message: i18n.T(c.Request().Context(), "validation.failed")})
	}

	if err := projects.ValidateProjectPage(c.Request().Context(), types.GetProjectPageInput{
		Search: request.Search, Page: request.Page, Size: request.Size,
	}); err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return c.JSON(http.StatusBadRequest, apiErr)
		}
		return err
	}
	var selectedIDs types.CommaSeparatedStrings
	if err := selectedIDs.UnmarshalParams(c.QueryParams()["projects"]); err != nil {
		return c.JSON(http.StatusBadRequest, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.invalid_selection")})
	}
	selected := make(map[string]bool, len(selectedIDs))
	uniqueIDs := make([]string, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != id {
			return c.JSON(http.StatusBadRequest, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.invalid_id")})
		}
		if !selected[id] {
			selected[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	var selectedItems []combobox.Option
	// First-page searches replace the list, so keep every selection in the DOM.
	// Later pages append only results that are not already selected.
	if request.Page == 0 && len(uniqueIDs) > 0 {
		selectedProjects, err := p.projectsService.GetProjectsByIds(c.Request().Context(), uniqueIDs)
		if err != nil {
			slog.Error("Error fetching selected projects", "error", err)
			return c.JSON(http.StatusInternalServerError, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.selected_load_failed")})
		}
		for _, project := range selectedProjects {
			selectedItems = append(selectedItems, combobox.Option{Value: project.ID.String(), Name: project.Name})
		}
	}

	results, err := p.projectsService.SearchProjects(c.Request().Context(), request)
	if err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return c.JSON(http.StatusBadRequest, apiErr)
		}
		slog.Error("Error fetching projects", "error", err)
		return c.JSON(http.StatusInternalServerError, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.load_failed")})
	}
	items := make([]combobox.Option, 0, len(results))
	for _, project := range results {
		if selected[project.ID.String()] {
			continue
		}
		items = append(items, combobox.Option{
			Value: project.ID.String(),
			Name:  project.Name,
		})
	}

	var nextUrl string

	if int(request.Size) == len(results) {
		url := *c.Request().URL
		q := url.Query()
		q.Set("page", fmt.Sprint(request.Page+1))
		// The inherited hx-include sends the current input, not this request's snapshot.
		q.Del("projects")
		url.RawQuery = q.Encode()
		nextUrl = url.String()
	}

	return utils.Render(c, http.StatusOK, combobox.ComboboxItems(combobox.ComboboxItemsProps{
		SelectedItems: selectedItems,
		Items:         items,
		NextURL:       nextUrl,
		Query:         request.Search,
	}))
}

func (p *ProjectsRouter) getProjects(c *echo.Context) error {
	var query types.GetProjectPageInput
	if err := c.Bind(&query); err != nil {
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}

	if query.Size == 0 {
		query.Size = 10
	}
	if err := utils.Validate(c, &query); err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return projectPageError(c, http.StatusBadRequest, apiErr.Message)
		}
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "projects.invalid_pagination"))
	}
	if err := projects.ValidateProjectPage(c.Request().Context(), query); err != nil {
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	return p.renderProjectsPage(c, query, false)
}

func (p *ProjectsRouter) getProject(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.id_required"), Fields: map[string]string{"id": i18n.T(c.Request().Context(), "projects.id_required")}})
	}
	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.load_one_failed")})
	}

	return c.JSON(200, project)
}

func (p *ProjectsRouter) getEditProjectForm(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.id_required")})
	}

	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.load_one_failed")})
	}

	return utils.Render(c, http.StatusOK, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
		FormID: "edit-project-form",
		Value: types.CreateProjectForm{
			ID:      project.ID.String(),
			Name:    project.Name,
			Clients: project.ClientNames(),
		},
	}))
}

func (p *ProjectsRouter) getConnectionStringForm(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.id_required")})
	}

	project, err := p.projectsService.GetProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching project", "error", err)
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.load_one_failed")})
	}

	return utils.Render(c, http.StatusOK, projectcomponents.ConnectionStringForm(projectcomponents.ConnectionStringFormProps{
		ProjectID: project.ID.String(),
		Clients:   project.Clients,
	}))
}

func (p *ProjectsRouter) createProject(c *echo.Context) error {
	query, err := projectMutationQuery(c)
	if err != nil {
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	var createProjectForm types.CreateProjectForm
	if err := c.Bind(&createProjectForm); err != nil {
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	if err := utils.Validate(c, &createProjectForm); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			htmx.Reswap(c, htmx.ReswapProps{
				Swap:   "outerHTML",
				Target: "this",
				Select: "form",
			})
			return utils.Render(c, http.StatusBadRequest, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
				ApiError: apiErr,
				Value:    createProjectForm,
			}))
		}

		return err
	}
	if err := projects.ValidateProjectForm(c.Request().Context(), createProjectForm); err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			htmx.Reswap(c, htmx.ReswapProps{Swap: "outerHTML", Target: "this", Select: "form"})
			return utils.Render(c, http.StatusBadRequest, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
				ApiError: apiErr, Value: createProjectForm,
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
			return utils.Render(c, http.StatusConflict, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
				ApiError: types.ApiError{
					Message: i18n.T(c.Request().Context(), "projects.already_exists"),
					Fields:  map[string]string{"name": i18n.T(c.Request().Context(), "projects.name_exists")}},
				Value: createProjectForm,
			}))
		}
		if err.Error() == projects.ErrProjectNotFound {
			return utils.Render(c, http.StatusNotFound, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
				ApiError: types.ApiError{Message: i18n.T(c.Request().Context(), "projects.not_found")}, Value: createProjectForm,
			}))
		}
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return utils.Render(c, http.StatusBadRequest, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
				ApiError: apiErr,
				Value:    createProjectForm,
			}))
		}

		slog.Error("Error creating project", "error", err)
		return utils.Render(c, http.StatusInternalServerError, projectcomponents.NewProjectForm(projectcomponents.NewProjectFormProps{
			ApiError: types.ApiError{
				Message: i18n.T(c.Request().Context(), "projects.save_failed"),
			},
			Value: createProjectForm,
		}))
	}

	htmx.CloseDialog(c)
	if createProjectForm.ID != "" {
		htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "projects.updated"))
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: fmt.Sprintf("[data-project-id='%s']", createProjectForm.ID),
			Select: "tr",
		})
	} else {
		htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "projects.created"))
	}
	if htmx.IsHXRequest(c) {
		return p.renderProjectsPage(c, query, true)
	}

	return utils.Render(c, http.StatusOK, admin.ProjectsTableBody(admin.ProjectsListPageProps{
		Projects: []projects.Project{project},
	}))
}

func (p *ProjectsRouter) deleteProject(c *echo.Context) error {
	query, err := projectMutationQuery(c)
	if err != nil {
		return projectPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.id_required"), Fields: map[string]string{"id": i18n.T(c.Request().Context(), "projects.id_required")}})
	}
	err = p.projectsService.DeleteProject(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error deleting project", "error", err)
		if err.Error() == projects.ErrProjectHasClients {
			message := i18n.T(c.Request().Context(), "projects.has_clients")
			htmx.AddErrorToast(c, message)
			c.Response().Header().Set("HX-Reswap", "none")
			return c.JSON(http.StatusConflict, types.ApiError{Message: message})
		}
		if err.Error() == projects.ErrProjectNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.delete_failed")})
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "projects.deleted"))
	if htmx.IsHXRequest(c) {
		return p.renderProjectsPage(c, query, true)
	}
	return c.HTML(200, "")
}

func (p *ProjectsRouter) createProjectConnString(c *echo.Context) error {
	var request serverauth.CredentialGenerationRequest
	if err := c.Bind(&request); err != nil {
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "connection.generate_failed"))
		return c.JSON(400, err)
	}
	if err := utils.Validate(c, &request); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			htmx.Reswap(c, htmx.ReswapProps{
				Swap:   "outerHTML",
				Target: "this",
				Select: "form",
			})

			project, projErr := p.projectsService.GetProject(c.Request().Context(), request.ProjectID)
			if projErr != nil {
				htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "connection.generate_failed"))
				return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.load_one_failed")})
			}

			return utils.Render(c, http.StatusBadRequest, projectcomponents.ConnectionStringForm(projectcomponents.ConnectionStringFormProps{
				ProjectID: request.ProjectID,
				Clients:   project.Clients,
				ApiError:  apiErr,
			}))
		}
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "connection.generate_failed"))
		return c.JSON(400, err)
	}

	creds, err := p.natsCredsService.GenerateConnString(c.Request().Context(), request)
	if err != nil {
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "connection.generate_failed"))
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "connection.generate_failed")})
	}

	connString := creds.GetConnString()
	slog.Debug("Generated connection string", "connString", connString)

	htmx.CloseDialog(c)
	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "connection.generated"))

	htmx.AddTrigger(c, "copyToClipboard", connString)

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
	group.GET("/search", router.searchProjects)
	group.GET("/:id", router.getProject)
	group.GET("/:id/edit", router.getEditProjectForm)
	group.GET("/:id/connection-string-form", router.getConnectionStringForm)
	group.POST("", router.createProject)
	group.POST("/conn-string", router.createProjectConnString)
	group.DELETE("/:id", router.deleteProject)

	return router
}
