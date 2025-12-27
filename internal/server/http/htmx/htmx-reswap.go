package htmx

import (
	"github.com/labstack/echo/v4"
)

type ReswapProps struct {
	Swap   string
	Target string
	Select string
}

func Reswap(c echo.Context, props ...ReswapProps) {
	var p ReswapProps
	if len(props) == 0 {
		p = ReswapProps{
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
	c.Response().Header().Set("HX-Retarget", p.Target)
	if p.Select != "" {
		c.Response().Header().Set("HX-Reselect", p.Select)
	}
}
