package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type archiveLinkVerifier interface {
	Verify(token string, now time.Time) (archivelink.Claims, error)
}

type archiveLinkUsers interface {
	middleware.ProjectAccessChecker
	GetUserByID(ctx context.Context, id string) (auth.User, error)
}

type archiveLinkRouter struct {
	links    archiveLinkVerifier
	users    archiveLinkUsers
	clients  middleware.ClientOwnershipChecker
	archiver archive.Archiver
}

// RegisterArchiveLinks serves signed download links issued to MCP clients.
// The link replaces the session, so the route is outside the session
// middleware, but it applies the same access rules as the archive UI.
func RegisterArchiveLinks(
	e *echo.Echo,
	links archiveLinkVerifier,
	users archiveLinkUsers,
	clients middleware.ClientOwnershipChecker,
	archiver archive.Archiver,
) {
	router := &archiveLinkRouter{links: links, users: users, clients: clients, archiver: archiver}
	e.GET(archivelink.Path+":token", router.download)
}

func (r *archiveLinkRouter) download(c *echo.Context) error {
	// The token in the URL must not leak to other sites or caches.
	header := c.Response().Header()
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "private, no-store")

	ctx := c.Request().Context()
	claims, err := r.links.Verify(c.Param("token"), time.Now())
	if err != nil {
		return archiveLinkInvalid(c)
	}
	allowed, err := r.allowed(ctx, claims)
	if err != nil {
		return err
	}
	if !allowed {
		return archiveLinkInvalid(c)
	}

	scope := archive.Scope{ProjectID: claims.ProjectID, ClientID: claims.ClientID}
	download, err := r.archiver.Open(ctx, scope, claims.ArchiveID)
	switch {
	case errors.Is(err, archive.ErrArchiveNotFound), errors.Is(err, archive.ErrClientNotFound), errors.Is(err, archive.ErrInvalidInput):
		return archiveLinkInvalid(c)
	case errors.Is(err, archive.ErrNotConfigured), errors.Is(err, archive.ErrArchiveUnavailable):
		return utils.Render(c, http.StatusServiceUnavailable, pages.ErrorPage(pages.ErrorPageProps{
			StatusCode: http.StatusServiceUnavailable,
			Message:    i18n.T(ctx, "archives.unavailable"),
		}))
	case err != nil:
		return err
	}
	return writeArchiveDownload(c, download, claims.ArchiveID)
}

// allowed re-checks the user's current access, as the archive UI does.
func (r *archiveLinkRouter) allowed(ctx context.Context, claims archivelink.Claims) (bool, error) {
	user, err := r.users.GetUserByID(ctx, claims.UserID.String())
	if errors.Is(err, auth.ErrUserNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Archives are limited to administrators, as in the web UI.
	if user.NeedsPasswordReset || user.Role != auth.ADMIN {
		return false, nil
	}
	hasAccess, err := r.users.UserHasAccessToProject(ctx, user.ID.String(), claims.ProjectID.String())
	if err != nil || !hasAccess {
		return false, err
	}
	return r.clients.ClientBelongsToProject(ctx, claims.ProjectID.String(), claims.ClientID.String())
}

func archiveLinkInvalid(c *echo.Context) error {
	return utils.Render(c, http.StatusNotFound, pages.ErrorPage(pages.ErrorPageProps{
		StatusCode: http.StatusNotFound,
		Message:    i18n.T(c.Request().Context(), "archives.download_link_invalid"),
	}))
}
