package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
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

	params := req.URL.Query()
	more := params.Has("older_cursor")
	p := pages.ArchiveSearchProps{
		ProjectID: scope.ProjectID.String(),
		ClientID:  scope.ClientID.String(),
		Text:      params.Get("text"),
	}
	if params.Has("instance") {
		instance := params.Get("instance")
		if len(params["instance"]) != 1 || logevent.ValidateInstanceID(instance) != nil {
			return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "logs.invalid_instances"))
		}
		p.Instance = &instance
	}
	render := func(status int) error {
		c.Response().Header().Set("Cache-Control", "private, no-store")
		if more {
			if p.ErrorKey != "" && status >= 500 {
				return archiveJSONError(c, status, i18n.T(ctx, p.ErrorKey))
			}
			return utils.Render(c, status, pages.ArchiveSearchMore(p))
		}
		if htmx.IsHXRequest(c) && req.Header.Get("HX-Request-Type") == "partial" {
			if status >= 500 {
				return archiveStatus(c, status, i18n.T(ctx, p.ErrorKey))
			}
			return utils.Render(c, status, pages.ArchiveSearchResults(p))
		}
		return utils.Render(c, status, pages.ArchiveSearch(p))
	}
	if !more && !params.Has("from") && !params.Has("to") && !params.Has("text") {
		return render(http.StatusOK)
	}
	if params.Get("from") == "" || params.Get("to") == "" {
		p.ErrorKey = "archives.range_required"
		return render(http.StatusBadRequest)
	}
	from, fromErr := strconv.ParseInt(params.Get("from"), 10, 64)
	to, toErr := strconv.ParseInt(params.Get("to"), 10, 64)
	// The calendar accepts four-digit years. This also rejects millisecond timestamps.
	limit := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if fromErr != nil || toErr != nil || from >= to || from < (time.Time{}).Unix() || to >= limit {
		p.ErrorKey = "logs.invalid_range"
		return render(http.StatusBadRequest)
	}
	p.FromUnix, p.ToUnix = strconv.FormatInt(from, 10), strconv.FormatInt(to, 10)
	query := archive.SearchQuery{From: time.Unix(from, 0).UTC(), To: time.Unix(to, 0).UTC(), Text: p.Text, Instance: p.Instance, OlderCursor: params.Get("older_cursor")}
	if more && (query.OlderCursor == "" || len(params["older_cursor"]) != 1) {
		p.ErrorKey = "archives.search_cursor_invalid"
		return render(http.StatusBadRequest)
	}
	status := http.StatusOK
	result, err := a.archiver.Search(ctx, scope, query)
	switch {
	case err == nil:
		p.Result = &result
		p.Query, _ = logsearch.Parse(p.Text)
	case errors.Is(err, logsearch.ErrTooShort):
		p.ErrorKey, status = "logs.search_too_short", http.StatusBadRequest
	case errors.Is(err, archive.ErrSearchCursor):
		p.ErrorKey, status = "archives.search_cursor_invalid", http.StatusConflict
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

	return render(status)
}
