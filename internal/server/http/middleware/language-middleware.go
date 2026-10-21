package middleware

import (
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

const LanguageCookieName = "svarog_language"

func LanguageMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		path := c.Request().URL.Path
		if path == "/assets" || strings.HasPrefix(path, "/assets/") {
			return next(c)
		}
		cookieValue := ""
		if cookie, err := c.Cookie(LanguageCookieName); err == nil {
			cookieValue = cookie.Value
		}
		lang := i18n.Resolve(cookieValue, c.Request().Header.Get("Accept-Language"))
		c.SetRequest(c.Request().WithContext(i18n.WithLanguage(c.Request().Context(), lang)))
		header := c.Response().Header()
		header.Set("Content-Language", string(lang))
		header.Add("Vary", "Cookie")
		header.Add("Vary", "Accept-Language")
		return next(c)
	}
}
