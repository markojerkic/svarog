package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	archivecomponents "github.com/markojerkic/svarog/internal/server/ui/components/archives"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type ArchiveRouter struct {
	archiver archive.Archiver
}

type archiveSettingsForm struct {
	ProjectID         string `param:"projectId"`
	ClientID          string `param:"clientId"`
	Enabled           string `form:"enabled"`
	ArchiveAfterWeeks string `form:"archiveAfterWeeks"`
}

func NewArchiveRouter(
	archiver archive.Archiver,
	authService auth.AuthService,
	projectsService projects.ProjectsService,
	e *echo.Group,
) *ArchiveRouter {
	router := &ArchiveRouter{archiver: archiver}
	group := e.Group(
		"/logs/:projectId/:clientId/archives",
		middleware.RequiresProjectAccessMiddleware(authService, projectsService),
		middleware.RequiresRoleMiddleware(auth.ADMIN),
	)
	group.GET("/settings", router.getSettings)
	group.POST("/settings", router.saveSettings)
	return router
}

func (a *ArchiveRouter) getSettings(c *echo.Context) error {
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}
	policy, err := a.archiver.GetPolicy(c.Request().Context(), scope)
	if err != nil {
		return archiveError(c, err, i18n.T(c.Request().Context(), "archives.load_failed"))
	}
	return utils.Render(c, http.StatusOK, archivecomponents.SettingsForm(archivecomponents.SettingsFormProps{
		ProjectID: scope.ProjectID.String(),
		ClientID:  scope.ClientID.String(),
		Policy:    policy,
	}))
}

func (a *ArchiveRouter) saveSettings(c *echo.Context) error {
	var form archiveSettingsForm
	if err := c.Bind(&form); err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	scope, err := archiveScope(form.ProjectID, form.ClientID)
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}

	policy := archive.Policy{Enabled: archiveFormEnabled(form.Enabled)}
	if policy.Enabled {
		weeks, err := strconv.Atoi(form.ArchiveAfterWeeks)
		if err != nil || weeks < 1 || weeks > archive.MaxArchiveAfterWeeks {
			return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "archives.invalid_weeks"))
		}
		policy.ArchiveAfterWeeks = weeks
	}

	if err := a.archiver.SetPolicy(c.Request().Context(), scope, policy); err != nil {
		return archiveError(c, err, i18n.T(c.Request().Context(), "archives.save_failed"))
	}
	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "archives.saved"))
	return utils.Render(c, http.StatusOK, archivecomponents.SettingsForm(archivecomponents.SettingsFormProps{
		ProjectID: scope.ProjectID.String(),
		ClientID:  scope.ClientID.String(),
		Policy:    policy,
	}))
}

func archiveFormEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "on", "yes":
		return true
	default:
		return false
	}
}

func archiveScope(projectID, clientID string) (archive.Scope, error) {
	project, err := uuid.Parse(projectID)
	if err != nil || project == uuid.Nil || project.String() != projectID {
		return archive.Scope{}, errors.New("invalid project ID")
	}
	client, err := uuid.Parse(clientID)
	if err != nil || client == uuid.Nil || client.String() != clientID {
		return archive.Scope{}, errors.New("invalid client ID")
	}
	return archive.Scope{ProjectID: project, ClientID: client}, nil
}

func archiveError(c *echo.Context, err error, fallback string) error {
	ctx := c.Request().Context()
	switch {
	case errors.Is(err, archive.ErrClientNotFound):
		htmx.AddErrorToast(c, i18n.T(ctx, "auth.project_client_missing"))
		return utils.HxRedirect(c, "/")
	case errors.Is(err, archive.ErrInvalidInput):
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(ctx, "archives.invalid_weeks"))
	case errors.Is(err, archive.ErrNotConfigured):
		return archiveJSONError(c, http.StatusServiceUnavailable, i18n.T(ctx, "archives.not_configured"))
	default:
		return archiveJSONError(c, http.StatusInternalServerError, fallback)
	}
}

func archiveJSONError(c *echo.Context, status int, message string) error {
	htmx.AddErrorToast(c, message)
	return c.JSON(status, message)
}
