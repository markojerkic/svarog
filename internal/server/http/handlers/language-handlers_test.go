package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/stretchr/testify/require"
)

func languageTestServer() *echo.Echo {
	e := echo.New()
	e.Use(middleware.LanguageMiddleware)
	handlers.NewLanguageRouter(e.Group(""))
	e.GET("/current", func(c *echo.Context) error {
		return c.String(http.StatusOK, i18n.T(c.Request().Context(), "common.cancel"))
	})
	return e
}

func TestLanguageMiddlewareUsesBrowserPreferenceUnlessCookieOverridesIt(t *testing.T) {
	e := languageTestServer()
	for _, test := range []struct{ cookie, body string }{
		{"", "Odustani"}, {"en", "Cancel"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/current", nil)
		req.Header.Set("Accept-Language", "hr-HR")
		req.AddCookie(&http.Cookie{Name: middleware.LanguageCookieName, Value: test.cookie})
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "Language negotiation must preserve a successful response")
		require.Equal(t, test.body, rec.Body.String(), "The explicit cookie must override the browser language in the rendering context")
	}
}

func TestChangeLanguagePersistsPreferenceAndRedirectsSafely(t *testing.T) {
	e := languageTestServer()
	for _, test := range []struct {
		lang, target, location string
		status                 int
	}{
		{"hr", "/current?search=%C4%8D&page=2", "/current?search=%C4%8D&page=2", http.StatusSeeOther},
		{"hr", "https://example.com", "/", http.StatusSeeOther},
		{"de", "/current", "", http.StatusBadRequest},
	} {
		form := url.Values{"language": {test.lang}, "returnTo": {test.target}}
		req := httptest.NewRequest(http.MethodPost, "/language", strings.NewReader(form.Encode()))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, test.status, rec.Code, "Valid choices must redirect with 303 and unsupported languages must be rejected")
		require.Equal(t, test.location, rec.Header().Get("Location"), "The return URL must preserve local filters but never redirect outside the app")
		cookies := rec.Result().Cookies()
		if test.status == http.StatusBadRequest {
			require.Empty(t, cookies, "Rejected choices must not overwrite the language cookie")
			continue
		}
		require.Len(t, cookies, 1, "A valid choice must set one preference cookie")
		require.Equal(t, test.lang, cookies[0].Value, "The preference must retain the selected language")
		require.True(t, cookies[0].HttpOnly, "Scripts must not need access to the preference cookie")
	}
}
