package middleware

import "github.com/labstack/echo/v5"

// NoStoreMiddleware marks private responses uncacheable. Apply it to the
// private group only, so /assets stay cacheable. The header is set when the
// status is written, and a handler that already set Cache-Control keeps its value.
func NoStoreMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		response, err := echo.UnwrapResponse(c.Response())
		if err != nil {
			return next(c)
		}
		response.Before(func() {
			header := c.Response().Header()
			if header.Get("Cache-Control") == "" {
				header.Set("Cache-Control", "private, no-store")
			}
		})
		return next(c)
	}
}
