package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/stretchr/testify/require"
)

type logoutAuthStub struct {
	auth.AuthService
}

func (logoutAuthStub) Logout(*echo.Context) error {
	return nil
}

func TestLogoutHXRedirectsToLogin(t *testing.T) {
	e := echo.New()
	handlers.NewAuthRouter(logoutAuthStub{}, nil, e.Group("/admin"), e.Group(""), e.Group(""))
	request := httptest.NewRequest(http.MethodGet, "/logout", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, "Logout of an HTMX request must return 200 so the client can read HX-Redirect")
	require.Equal(t, "/login", response.Header().Get("HX-Redirect"), "Logout must fully navigate to login instead of ajax-hopping through home")
	require.Empty(t, response.Header().Get("HX-Location"), "Logout must not use HX-Location, which swaps content and leaves the address bar on the private URL")
}

func TestPrivateHTMLIsNotStored(t *testing.T) {
	e := echo.New()
	user := &auth.LoggedInUser{Role: auth.USER}
	group := e.Group("", middleware.NoStoreMiddleware, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(auth.UserKey.String(), user)
			return next(c)
		}
	})
	handlers.NewHomeHandler(group, nil)

	response := httptest.NewRecorder()
	e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, response.Code, "A logged-in home request must render the private page")
	require.Contains(t, response.Body.String(), "No projects available", "The private home response must be HTML rather than a redirect")
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"), "Private HTML must not be stored, so Back cannot restore logs or admin chrome from the cache")
}

func TestNoStoreDoesNotOverrideHandlerCacheControl(t *testing.T) {
	e := echo.New()
	e.GET("/download", func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private, max-age=0")
		return c.HTML(http.StatusOK, "<html></html>")
	}, middleware.NoStoreMiddleware)

	response := httptest.NewRecorder()
	e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/download", nil))

	require.Equal(t, http.StatusOK, response.Code, "A handler that sets its own cache policy must still respond")
	require.Equal(t, "private, max-age=0", response.Header().Get("Cache-Control"), "No-store middleware must not override a handler that already set Cache-Control")
}
