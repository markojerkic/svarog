package htmx

import (
	"github.com/labstack/echo/v4"
)

func CloseDialog(c echo.Context) {
	if c.Request().Header.Get("HX-Request") != "true" {
		return
	}
	c.Response().Header().Set("close-dialog", "true")
}
