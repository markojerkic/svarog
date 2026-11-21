package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func (a *ArchiveRouter) searchArchives(c *echo.Context) error {
	req := c.Request()
	ctx := req.Context()
	scope, err := archiveScope(c.Param("projectId"), c.Param("clientId"))
	if err != nil {
		return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "auth.invalid_ids"))
	}

	p := pages.ArchiveSearchProps{
		ProjectID: scope.ProjectID.String(),
		ClientID:  scope.ClientID.String(),
	}
	if req.Method == http.MethodGet {
		return utils.Render(c, http.StatusOK, pages.ArchiveSearch(p))
	}

	req.Body = http.MaxBytesReader(c.Response(), req.Body, 8<<10)
	if err := req.ParseForm(); err != nil {
		return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "request.invalid_parameters"))
	}

	p.FromLocal = req.PostForm.Get("fromLocal")
	p.ToLocal = req.PostForm.Get("toLocal")
	p.Text = req.PostForm.Get("text")

	from, fromErr := time.Parse(time.RFC3339Nano, req.PostForm.Get("fromAt"))
	to, toErr := time.Parse(time.RFC3339Nano, req.PostForm.Get("toAt"))
	if fromErr != nil || toErr != nil || !to.After(from) {
		p.ErrorKey = "logs.invalid_range"
		return utils.Render(c, http.StatusBadRequest, pages.ArchiveSearch(p))
	}

	status := http.StatusOK
	result, err := a.archiver.Search(ctx, scope, archive.SearchQuery{From: from, To: to, Text: p.Text})
	switch {
	case err == nil:
		p.Result = &result
		p.Query, _ = logsearch.Parse(p.Text)
	case errors.Is(err, logsearch.ErrTooShort):
		p.ErrorKey, status = "logs.search_too_short", http.StatusBadRequest
	case errors.Is(err, archive.ErrInvalidInput):
		p.ErrorKey, status = "request.invalid_parameters", http.StatusBadRequest
	case errors.Is(err, archive.ErrSearchLimit), errors.Is(err, context.DeadlineExceeded):
		p.ErrorKey, status = "archives.search_limit", http.StatusUnprocessableEntity
	case errors.Is(err, context.Canceled):
		return err
	default:
		slog.ErrorContext(ctx, "Archive search failed", "clientID", scope.ClientID, "error", err)
		p.ErrorKey, status = "archives.search_failed", http.StatusServiceUnavailable
	}

	return utils.Render(c, status, pages.ArchiveSearch(p))
}
