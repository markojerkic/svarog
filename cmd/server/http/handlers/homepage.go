package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
	"github.com/markojerkic/svarog/cmd/server/views"
)

type handler func(db.LogRepository) echo.HandlerFunc

func HomePage(logRepository db.LogRepository) echo.HandlerFunc {
	clients, err := logRepository.GetClients()
	return func(c echo.Context) error {
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return views.Clients(clients).Render(c.Request().Context(), c.Response().Writer)
	}
}
