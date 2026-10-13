package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/stretchr/testify/require"
)

func TestUsersWithoutProjectAssignmentsCannotReadProjectData(t *testing.T) {
	e := echo.New()
	group := e.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(auth.UserKey.String(), &auth.LoggedInUser{Role: auth.USER})
			return next(c)
		}
	})
	// Nil persistence/hub dependencies ensure denied requests never reach them.
	handlers.NewHomeHandler(group, nil)
	handlers.NewLogsRouter(nil, group)
	handlers.NewWsConnectionRouter(nil, group)
	for _, path := range []string{
		"/logs/project/client", "/logs/project/client/search", "/logs/project/client/instances", "/ws/project/client",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, "/", rec.Header().Get("HX-Location"), "Users without project assignments should be redirected home from %s", path)
			require.Empty(t, rec.Body.String(), "Denied requests to %s should not return project data", path)
		})
	}
	t.Run("home does not query projects", func(t *testing.T) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, rec.Code, "The home page should load for users without project assignments without querying projects")
	})
}
