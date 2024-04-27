package http

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
	"github.com/markojerkic/svarog/server/views"
)

type HttpServer struct {
	logRepository db.LogRepository
}

func NewServer(logRepository db.LogRepository) *HttpServer {
	return &HttpServer{
		logRepository: logRepository,
	}
}

func (self *HttpServer) Start(port int) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		clients, err := self.logRepository.GetClients()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		return views.Logs(clients).Render(c.Request().Context(), c.Response().Writer)

	})
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}
