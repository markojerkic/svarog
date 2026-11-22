package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/components/archives"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func (a *ArchiveRouter) archiveHitContext(c *echo.Context) error {
	ctx := c.Request().Context()
	project, client := c.Param("projectId"), c.Param("clientId")
	scope, err := archiveScope(project, client)
	if err != nil {
		return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "auth.invalid_ids"))
	}
	id, err := uuid.Parse(c.Param("archiveId"))
	if err != nil || id == uuid.Nil || id.String() != c.Param("archiveId") {
		return archiveStatus(c, http.StatusNotFound, i18n.T(ctx, "archives.not_found"))
	}
	params := c.Request().URL.Query()
	line, err := strconv.Atoi(params.Get("line"))
	if err != nil || line < 1 || params.Get("entry") == "" {
		return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "request.invalid_parameters"))
	}
	lines := archive.DefaultContextLines
	if params.Has("lines") {
		lines, err = strconv.Atoi(params.Get("lines"))
		if err != nil || lines < 1 || lines > archive.MaxContextLines {
			return archiveStatus(c, http.StatusBadRequest, i18n.T(ctx, "request.invalid_parameters"))
		}
	}
	ref := archive.HitRef{ArchiveID: id, Entry: params.Get("entry"), Line: line}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	window, err := a.archiver.Context(ctx, scope, ref, lines)
	switch {
	case errors.Is(err, archive.ErrHitNotFound):
		return archiveStatus(c, http.StatusNotFound, i18n.T(ctx, "archives.hit_missing"))
	case errors.Is(err, archive.ErrSearchLimit), errors.Is(err, context.DeadlineExceeded):
		return archiveStatus(c, http.StatusUnprocessableEntity, i18n.T(ctx, "archives.context_limit"))
	case errors.Is(err, context.Canceled):
		return err
	case err != nil:
		slog.WarnContext(ctx, "Cannot read archive context", "archive", id, "err", err)
		return archiveDownloadError(c, err)
	}
	if strings.HasSuffix(c.Path(), "/download") {
		var text bytes.Buffer
		for _, hit := range window.Rows {
			fmt.Fprintf(&text, "[%s %s] %s\n", hit.OccurredAt.UTC().Format(time.RFC3339Nano), hit.Instance, hit.Message)
		}
		filename := fmt.Sprintf("context-%s-line-%d.log", id, line)
		c.Response().Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.Blob(http.StatusOK, "text/plain; charset=utf-8", text.Bytes())
	}
	back := url.Values{}
	for _, name := range []string{"from", "to", "text", "instance"} {
		if params.Has(name) {
			back.Set(name, params.Get(name))
		}
	}
	path := archives.ArchiveDownloadURL(project, client, id.String())
	return utils.Render(c, http.StatusOK, pages.ArchiveContext(pages.ArchiveContextProps{
		Window: window, Ref: ref, Lines: lines, ArchivePath: path, BackParams: back,
		SearchPath: archives.ArchivePanelURL(project, client) + "/search",
	}))
}
