package handlers_test

import (
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	serverhttp "github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/stretchr/testify/require"
)

type loginLinkAuthStub struct {
	auth.AuthService
	tokenLogins    []string
	passwordLogins []string
	tokenErr       error
}

func (s *loginLinkAuthStub) Login(_ *echo.Context, username, _ string) error {
	s.passwordLogins = append(s.passwordLogins, username)
	return nil
}

func (s *loginLinkAuthStub) LoginWithToken(_ *echo.Context, token string) error {
	s.tokenLogins = append(s.tokenLogins, token)
	return s.tokenErr
}

func newLoginLinkServer(service auth.AuthService) *echo.Echo {
	e := echo.New()
	e.Validator = serverhttp.NewValidator()
	e.Use(middleware.LanguageMiddleware)
	handlers.NewAuthRouter(service, nil, e.Group("/admin"), e.Group(""), e.Group(""))
	return e
}

func TestLoginLinkIsConsumedOnlyByPost(t *testing.T) {
	const token = `one-time-token&<"quote"`

	t.Run("get renders a confirmation form and does not spend the token", func(t *testing.T) {
		stub := &loginLinkAuthStub{}
		request := httptest.NewRequest(http.MethodGet, "/login?token="+url.QueryEscape(token), nil)
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		body := response.Body.String()
		require.Equal(t, http.StatusOK, response.Code, "Opening a login link must render a confirmation page instead of signing in")
		require.Empty(t, stub.tokenLogins, "GET /login?token= must not consume or validate the token by logging in")
		require.Empty(t, stub.passwordLogins, "A login link must not fall through to password login")
		require.Empty(t, response.Result().Cookies(), "GET /login?token= must not create a session")
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"), "The confirmation page contains the login token and must not be cached")
		require.Equal(t, "no-referrer", response.Header().Get("Referrer-Policy"), "The token in the login URL must not leak through the Referer header")
		require.Contains(t, body, `action="/login/token"`, "The confirmation button must submit the token to the POST endpoint")
		require.Contains(t, body, `hx-post="/login/token"`, "The confirmation form must POST the token rather than navigate with GET")
		require.Contains(t, body, `name="token" value="`+html.EscapeString(token)+`"`, "The unused token must stay in a hidden field, escaped")
		require.NotContains(t, body, token, "A token with markup must not be written into HTML unescaped")
		require.Contains(t, body, ">Sign in<", "The confirmation button must offer sign-in")
		require.NotContains(t, body, `name="password"`, "The login-link page must not ask for a password")
	})

	t.Run("croatian confirmation uses the login button label", func(t *testing.T) {
		stub := &loginLinkAuthStub{}
		request := httptest.NewRequest(http.MethodGet, "/login?token="+url.QueryEscape(token), nil)
		request.Header.Set("Accept-Language", "hr")
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code, "A Croatian login link must still render without consuming the token")
		require.Empty(t, stub.tokenLogins, "Language selection must not consume the login token")
		require.Contains(t, response.Body.String(), ">Prijavi se<", "The Croatian confirmation button must say Prijavi se")
		require.NotContains(t, response.Body.String(), "Prijavite se", "The login-link button must not reuse the password form label")
	})

	t.Run("post consumes the token and redirects to password reset", func(t *testing.T) {
		stub := &loginLinkAuthStub{}
		form := url.Values{"token": {token}}
		request := httptest.NewRequest(http.MethodPost, "/login/token", strings.NewReader(form.Encode()))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		request.Header.Set("HX-Request", "true")
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code, "An HTMX token login must return 200 so the client can read the redirect and toast")
		require.Equal(t, []string{token}, stub.tokenLogins, "POST /login/token must consume the submitted token exactly once")
		require.Equal(t, "/reset-password", response.Header().Get("HX-Redirect"), "A successful token login must continue to password reset")
		require.JSONEq(t, `{"toast":{"level":"success","message":"Logged in"}}`, response.Header().Get("HX-Trigger"), "A successful token login must show the existing success toast")
		require.Empty(t, stub.passwordLogins, "Token login must not use password authentication")
	})

	t.Run("post without htmx still consumes the token and does not replay the post", func(t *testing.T) {
		stub := &loginLinkAuthStub{}
		form := url.Values{"token": {token}}
		request := httptest.NewRequest(http.MethodPost, "/login/token", strings.NewReader(form.Encode()))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		require.Equal(t, http.StatusSeeOther, response.Code, "A non-HTMX token login must redirect with 303 so the browser GETs the next page")
		require.Equal(t, "/reset-password", response.Header().Get("Location"), "A non-HTMX token login must continue to password reset")
		require.Equal(t, []string{token}, stub.tokenLogins, "A non-HTMX POST must still consume the token")
	})

	t.Run("invalid token does not log the user in", func(t *testing.T) {
		stub := &loginLinkAuthStub{tokenErr: errors.New(auth.LoginTokenNotValid)}
		form := url.Values{"token": {token}}
		request := httptest.NewRequest(http.MethodPost, "/login/token", strings.NewReader(form.Encode()))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		request.Header.Set("HX-Request", "true")
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		require.Equal(t, http.StatusOK, response.Code, "An invalid token must return the existing HTMX error redirect, not a session")
		require.Equal(t, []string{token}, stub.tokenLogins, "An invalid token must be rejected by login rather than ignored")
		require.Equal(t, "/login", response.Header().Get("HX-Redirect"), "An invalid or already-used token must return the user to login")
		require.JSONEq(t, `{"toast":{"level":"error","message":"Token is invalid"}}`, response.Header().Get("HX-Trigger"), "An invalid token must use the existing invalid-token response")
		require.Empty(t, response.Result().Cookies(), "An invalid token must not create a session")
	})

	t.Run("password login stays on POST /login", func(t *testing.T) {
		stub := &loginLinkAuthStub{}
		form := url.Values{"username": {"admin"}, "password": {"ADMINADMIN"}}
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		response := httptest.NewRecorder()
		newLoginLinkServer(stub).ServeHTTP(response, request)

		require.Equal(t, http.StatusFound, response.Code, "Password login must keep its existing redirect")
		require.Equal(t, "/", response.Header().Get("Location"), "Password login must still redirect home")
		require.Equal(t, []string{"admin"}, stub.passwordLogins, "POST /login must remain password login")
		require.Empty(t, stub.tokenLogins, "Password login must not consume a login-link token")
	})
}
