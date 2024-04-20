package http

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
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
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}
