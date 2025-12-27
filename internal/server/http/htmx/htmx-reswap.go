package htmx

import (
	"github.com/labstack/echo/v4"
)

type ErrorReswapProps struct {
	Swap   string
	Target string
}

func ErrorReswap(c echo.Context, props ...ErrorReswapProps) {
	var p ErrorReswapProps
	if len(props) == 0 {
		p = ErrorReswapProps{
			Swap:   "outerHTML",
			Target: "this",
		}
	} else {
		p = props[0]
	}

	if c.Request().Header.Get("HX-Request") != "true" {
		return
	}

	c.Response().Header().Set("HX-Reswap", p.Swap)
	c.Response().Header().Set("HX-Target", p.Target)
}
