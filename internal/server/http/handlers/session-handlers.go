package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/forms"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	authpages "github.com/markojerkic/svarog/internal/server/ui/pages/auth"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type loginService interface {
	Login(ctx *echo.Context, username, password string) error
	LoginWithToken(ctx *echo.Context, token string) error
}

type passwordService interface {
	middleware.CurrentUserReader
	ResetPassword(ctx context.Context, userID, password string) error
}

type logoutService interface {
	Logout(*echo.Context) error
}

type loginRouter struct {
	authService loginService
}

type passwordRouter struct {
	authService passwordService
}

func RegisterLogin(service loginService, public *echo.Group) {
	router := &loginRouter{authService: service}
	public.GET("/login", func(c *echo.Context) error {
		if c.QueryParams().Has("token") {
			return router.loginTokenPage(c)
		}
		return router.loginPage(c)
	})
	public.POST("/login", router.login)
	public.POST("/login/token", router.loginWithToken)
}

func RegisterPasswordReset(service passwordService, private *echo.Group) {
	router := &passwordRouter{authService: service}
	private.GET("/reset-password", router.resetPasswordPage)
	private.POST("/reset-password", router.resetPassword)
}

func RegisterLogout(service logoutService, private *echo.Group) {
	private.GET("/logout", func(c *echo.Context) error {
		err := service.Logout(c)
		if err != nil {
			return c.JSON(500, forms.ApiError{Message: i18n.T(c.Request().Context(), "auth.logout_failed")})
		}
		return htmx.Redirect(c, "/login")
	})
}

func loginRedirect(target string) string {
	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil {
		return "/"
	}
	if !strings.HasPrefix(target, "/") || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") {
		return "/"
	}
	return target
}

func (a *loginRouter) login(c *echo.Context) error {
	var loginForm forms.LoginForm
	bindErr := c.Bind(&loginForm)
	redirect := loginForm.Redirect
	if redirect == "" {
		redirect = c.QueryParam("redirect")
	}
	redirect = loginRedirect(redirect)
	if bindErr != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(authpages.LoginPageProps{
			Error:    i18n.T(c.Request().Context(), "auth.invalid_form"),
			Username: loginForm.Username,
			Password: loginForm.Password,
			Redirect: redirect,
		}))
	}
	if err := utils.Validate(c, &loginForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    i18n.T(c.Request().Context(), "auth.credentials_required"),
				Username: loginForm.Username,
				Password: loginForm.Password,
				Redirect: redirect,
			},
		))
	}

	err := a.authService.Login(c, loginForm.Username, loginForm.Password)
	if err != nil {
		return utils.Render(c, http.StatusUnauthorized, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    i18n.T(c.Request().Context(), "auth.invalid_credentials"),
				Username: loginForm.Username,
				Password: loginForm.Password,
				Redirect: redirect,
			},
		))
	}

	return utils.HxRedirect(c, redirect)
}

func (a *loginRouter) loginWithToken(c *echo.Context) error {
	var loginForm forms.LoginFormWithToken
	if err := c.Bind(&loginForm); err != nil {
		return c.JSON(400, err)
	}
	if err := utils.Validate(c, &loginForm); err != nil {
		return err
	}

	err := a.authService.LoginWithToken(c, loginForm.Token)
	if err != nil {
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.token_invalid"))
		return redirectAfterTokenLogin(c, "/login")
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "auth.logged_in"))
	return redirectAfterTokenLogin(c, "/reset-password")
}

// A 307 would replay POST /login/token onto the next page. HTMX keeps the
// existing toast plus HX-Redirect response.
func redirectAfterTokenLogin(c *echo.Context, target string) error {
	if htmx.IsHXRequest(c) {
		return htmx.Redirect(c, target)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// Preview clients fetch the login link before the user opens it. Rendering a
// confirmation form keeps that GET from spending the single-use token.
func (a *loginRouter) loginTokenPage(c *echo.Context) error {
	header := c.Response().Header()
	header.Set("Cache-Control", "private, no-store")
	header.Set("Referrer-Policy", "no-referrer")
	return utils.Render(c, http.StatusOK, authpages.LoginTokenPage(authpages.LoginTokenPageProps{
		Token: c.QueryParam("token"),
	}))
}

func (a *loginRouter) loginPage(c *echo.Context) error {
	return utils.Render(c, http.StatusOK, authpages.LoginPage(authpages.LoginPageProps{
		Redirect: loginRedirect(c.QueryParam("redirect")),
	}))
}

func (a *passwordRouter) resetPasswordPage(c *echo.Context) error {
	var redirect string
	if c.QueryParams().Has("redirect") {
		redirect = loginRedirect(c.QueryParam("redirect"))
	}
	return utils.Render(c, http.StatusOK, authpages.ResetPasswordPage(authpages.ResetPasswordPageProps{
		Redirect: redirect,
	}))
}

func (a *passwordRouter) resetPassword(c *echo.Context) error {
	var resetPasswordForm forms.ResetPasswordForm
	if err := c.Bind(&resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.invalid_form")))
	}
	if err := utils.Validate(c, &resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.password_too_short")))
	}

	if resetPasswordForm.Password != resetPasswordForm.RepeatedPassword {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.passwords_mismatch")))
	}
	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		// Preserve the form error when a submitted password is invalid and the session lookup fails.
		if err := auth.ValidateNewPassword(resetPasswordForm.Password); err != nil {
			return resetPasswordError(c, err)
		}
		return htmx.Redirect(c, "/login")
	}

	err = a.authService.ResetPassword(c.Request().Context(), user.ID, resetPasswordForm.Password)
	if err != nil {
		return resetPasswordError(c, err)
	}

	// The return target must stay on Svarog, as after login.
	return utils.HxRedirect(c, loginRedirect(c.FormValue("redirect")))
}

func resetPasswordError(c *echo.Context, err error) error {
	status := http.StatusBadRequest
	var key string
	switch {
	case errors.Is(err, auth.ErrPasswordBlank):
		key = "auth.password_blank"
	case errors.Is(err, auth.ErrPasswordTooLong):
		key = "auth.password_too_short"
	default:
		slog.Error("Error resetting password", "error", err)
		status = http.StatusInternalServerError
		key = "auth.reset_failed"
	}
	return utils.Render(c, status, authpages.ResetPasswordError(i18n.T(c.Request().Context(), key)))
}
