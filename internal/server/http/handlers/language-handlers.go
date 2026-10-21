package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

func changeLanguage(c *echo.Context) error {
	lang := i18n.Language(c.FormValue("language"))
	if lang != i18n.English && lang != i18n.Croatian {
		return c.String(http.StatusBadRequest, i18n.T(c.Request().Context(), "language.invalid"))
	}
	const maxAge = 365 * 24 * 60 * 60
	c.SetCookie(&http.Cookie{
		Name:     middleware.LanguageCookieName,
		Value:    string(lang),
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  time.Now().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: true,
		Secure:   c.Scheme() == "https",
		SameSite: http.SameSiteLaxMode,
	})
	c.Response().Header().Set("Content-Language", string(lang))
	target := loginRedirect(c.FormValue("returnTo"))
	if htmx.IsHXRequest(c) {
		return htmx.Redirect(c, target)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

func NewLanguageRouter(public *echo.Group) {
	public.POST("/language", changeLanguage)
}
