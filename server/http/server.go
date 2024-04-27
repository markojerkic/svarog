package http

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
	"github.com/markojerkic/svarog/server/http/handlers"
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

	e.GET("/", handlers.HomePage(self.logRepository))

	e.GET("/logs/:clientId", handlers.LogsByClient(self.logRepository))

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}
