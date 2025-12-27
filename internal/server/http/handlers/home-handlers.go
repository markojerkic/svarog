package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type HomeHandler struct {
	group *echo.Group
}

func NewHomeHandler(group *echo.Group) *HomeHandler {
	handler := &HomeHandler{group}
	handler.registerRoutes()
	return handler
}

func (self *HomeHandler) registerRoutes() {
	self.group.GET("/", func(c echo.Context) error {
		return utils.Render(c, http.StatusOK, pages.Homepage())
	})
}
