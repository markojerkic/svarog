package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func RequiresProjectAccessMiddleware(authService auth.AuthService, projectsService projects.ProjectsService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
			if !ok || user == nil {
				htmx.AddErrorToast(c, "You are not logged in")
				return utils.HxRedirect(c, "/login")
			}

			projectID, clientID := c.Param("projectId"), c.Param("clientId")
			for _, id := range []string{projectID, clientID} {
				parsed, err := uuid.Parse(id)
				if err != nil || parsed == uuid.Nil || parsed.String() != id {
					return c.JSON(http.StatusBadRequest, types.ApiError{Message: "Project and client IDs must be canonical UUIDs"})
				}
			}

			ctx := c.Request().Context()
			allowed, err := authService.UserHasAccessToProject(ctx, user.ID, projectID)
			if err != nil {
				return err
			}
			if !allowed {
				htmx.AddErrorToast(c, "You don't have permission to access this project")
				return utils.HxRedirect(c, "/")
			}

			// Historical logs remain readable for inactive clients, but never for another project's client.
			belongs, err := projectsService.ClientBelongsToProject(ctx, projectID, clientID)
			if err != nil {
				return err
			}
			if !belongs {
				htmx.AddErrorToast(c, "Project or client not found")
				return utils.HxRedirect(c, "/")
			}
			return next(c)
		}
	}
}
