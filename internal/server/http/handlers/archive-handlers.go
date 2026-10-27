package handlers

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	archivecomponents "github.com/markojerkic/svarog/internal/server/ui/components/archives"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type ArchiveRouter struct {
	archiver archive.Archiver
}

type archiveSettingsForm struct {
	ProjectID         string         `param:"projectId"`
	ClientID          string         `param:"clientId"`
	Enabled           types.FormBool `form:"enabled"`
	ArchiveAfterWeeks string         `form:"archiveAfterWeeks"`
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
	group.GET("", router.getPanel)
	group.GET("/settings", router.getSettings)
	group.POST("/settings", router.saveSettings)
	group.POST("/create", router.createArchive)
	group.GET("/list", router.listArchives)
	group.GET("/:archiveId", router.downloadArchive)
	return router
}

func (a *ArchiveRouter) getPanel(c *echo.Context) error {
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}
	policy, err := a.archiver.GetPolicy(c.Request().Context(), scope)
	if err != nil {
		return archiveError(c, err, i18n.T(c.Request().Context(), "archives.load_failed"))
	}
	page, err := a.archiver.List(c.Request().Context(), scope, archive.ListQuery{})
	if err != nil {
		return archiveError(c, err, i18n.T(c.Request().Context(), "archives.list_failed"))
	}
	return utils.Render(c, http.StatusOK, archivecomponents.Panel(archivecomponents.PanelProps{
		Settings: archivecomponents.SettingsFormProps{
			ProjectID: scope.ProjectID.String(),
			ClientID:  scope.ClientID.String(),
			Policy:    policy,
		},
		Page: page,
	}))
}

func (a *ArchiveRouter) listArchives(c *echo.Context) error {
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}
	cursor, err := archiveCursor(c.QueryParam("before"), c.QueryParam("beforeId"))
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "archives.invalid_cursor"))
	}
	page, err := a.archiver.List(c.Request().Context(), scope, archive.ListQuery{Before: cursor})
	if err != nil {
		return archiveError(c, err, i18n.T(c.Request().Context(), "archives.list_failed"))
	}
	return utils.Render(c, http.StatusOK, archivecomponents.ArchiveListMore(
		scope.ProjectID.String(),
		scope.ClientID.String(),
		page,
	))
}

func (a *ArchiveRouter) createArchive(c *echo.Context) error {
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveJSONError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}
	ctx := c.Request().Context()
	result, err := a.archiver.Create(ctx, scope)
	if err != nil {
		slog.ErrorContext(ctx, "Manual archiving failed", "projectID", scope.ProjectID, "clientID", scope.ClientID, "error", err)
		return archiveError(c, err, i18n.T(ctx, "archives.create_failed"))
	}
	switch result.Outcome {
	case archive.Created:
		page, err := a.archiver.List(ctx, scope, archive.ListQuery{})
		if err != nil {
			slog.ErrorContext(ctx, "Could not refresh archives after creation", "clientID", scope.ClientID, "error", err)
			return archiveJSONError(c, http.StatusInternalServerError, i18n.T(ctx, "archives.created_list_failed"))
		}
		htmx.AddSuccessToast(c, i18n.T(ctx, "archives.created"))
		return utils.Render(c, http.StatusOK, archivecomponents.ArchiveList(scope.ProjectID.String(), scope.ClientID.String(), page))
	case archive.SkippedNoLogs:
		htmx.AddInfoToast(c, i18n.T(ctx, "archives.no_eligible_logs"))
	case archive.SkippedDisabled:
		htmx.AddInfoToast(c, i18n.T(ctx, "archives.enable_first"))
	case archive.SkippedBusy:
		htmx.AddInfoToast(c, i18n.T(ctx, "archives.already_running"))
	default:
		slog.ErrorContext(ctx, "Unexpected manual archive outcome", "clientID", scope.ClientID, "outcome", result.Outcome)
		return archiveJSONError(c, http.StatusInternalServerError, i18n.T(ctx, "archives.create_failed"))
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *ArchiveRouter) downloadArchive(c *echo.Context) error {
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveStatus(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "auth.invalid_ids"))
	}
	archiveID, err := uuid.Parse(c.Param("archiveId"))
	if err != nil || archiveID == uuid.Nil || archiveID.String() != c.Param("archiveId") {
		return archiveStatus(c, http.StatusNotFound, i18n.T(c.Request().Context(), "archives.not_found"))
	}
	download, err := a.archiver.Open(c.Request().Context(), scope, archiveID)
	if err != nil {
		return archiveDownloadError(c, err)
	}
	defer download.Content.Close()

	name := archiveAttachmentName(download.Archive.FileName, archiveID.String())
	header := c.Response().Header()
	header.Set("Content-Type", "application/zip")
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "private, no-store")
	if download.Archive.SizeBytes >= 0 {
		header.Set("Content-Length", strconv.FormatInt(download.Archive.SizeBytes, 10))
	}
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), download.Content)
	return err
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

	policy := archive.Policy{Enabled: form.Enabled.Bool()}
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
	return utils.Render(c, http.StatusOK, archivecomponents.SettingsSaved(archivecomponents.SettingsFormProps{
		ProjectID: scope.ProjectID.String(),
		ClientID:  scope.ClientID.String(),
		Policy:    policy,
	}))
}

func archiveCursor(before, id string) (*archive.Cursor, error) {
	if before == "" && id == "" {
		return nil, nil
	}
	createdAt, err := time.Parse(time.RFC3339Nano, before)
	parsed, idErr := uuid.Parse(id)
	if err != nil || idErr != nil || parsed == uuid.Nil || parsed.String() != id {
		return nil, errors.New("invalid archive cursor")
	}
	return &archive.Cursor{CreatedAt: createdAt, ID: parsed}, nil
}

func archiveAttachmentName(name, fallback string) string {
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	base = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' {
			return -1
		}
		return r
	}, base)
	if base == "" || base == "." || base == ".." {
		return fallback + ".zip"
	}
	return base
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

func archiveDownloadError(c *echo.Context, err error) error {
	ctx := c.Request().Context()
	switch {
	case errors.Is(err, archive.ErrClientNotFound):
		return archiveError(c, err, i18n.T(ctx, "archives.not_found"))
	case errors.Is(err, archive.ErrArchiveNotFound), errors.Is(err, archive.ErrInvalidInput):
		return archiveStatus(c, http.StatusNotFound, i18n.T(ctx, "archives.not_found"))
	case errors.Is(err, archive.ErrNotConfigured), errors.Is(err, archive.ErrArchiveUnavailable):
		return archiveStatus(c, http.StatusServiceUnavailable, i18n.T(ctx, "archives.unavailable"))
	default:
		return archiveStatus(c, http.StatusInternalServerError, i18n.T(ctx, "archives.unavailable"))
	}
}

func archiveStatus(c *echo.Context, status int, message string) error {
	if htmx.IsHXRequest(c) {
		htmx.AddErrorToast(c, message)
		return c.NoContent(status)
	}
	return c.String(status, message)
}
