package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/forms"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

type ProjectAccessChecker interface {
	UserHasAccessToProject(ctx context.Context, userID, projectID string) (bool, error)
}

type ClientOwnershipChecker interface {
	ClientBelongsToProject(ctx context.Context, projectID, clientID string) (bool, error)
}

func RequiresProjectAccessMiddleware(authService ProjectAccessChecker, projectsService ClientOwnershipChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
			if !ok || user == nil {
				htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.login_required"))
				return htmx.Redirect(c, "/login")
			}

			projectID, clientID := c.Param("projectId"), c.Param("clientId")
			for _, id := range []string{projectID, clientID} {
				parsed, err := uuid.Parse(id)
				if err != nil || parsed == uuid.Nil || parsed.String() != id {
					return c.JSON(http.StatusBadRequest, forms.ApiError{Message: i18n.T(c.Request().Context(), "auth.invalid_ids")})
				}
			}

			ctx := c.Request().Context()
			allowed, err := authService.UserHasAccessToProject(ctx, user.ID, projectID)
			if err != nil {
				return err
			}
			if !allowed {
				htmx.AddErrorToast(c, i18n.T(ctx, "auth.project_forbidden"))
				return htmx.Redirect(c, "/")
			}

			// Historical logs remain readable for inactive clients, but never for another project's client.
			belongs, err := projectsService.ClientBelongsToProject(ctx, projectID, clientID)
			if err != nil {
				return err
			}
			if !belongs {
				htmx.AddErrorToast(c, i18n.T(ctx, "auth.project_client_missing"))
				return htmx.Redirect(c, "/")
			}
			return next(c)
		}
	}
}
