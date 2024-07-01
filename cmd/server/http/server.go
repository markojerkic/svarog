package http

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markojerkic/svarog/cmd/server/http/handlers"
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

	api := e.Group("/api/v1")
	api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
	}))

	api.GET("/clients", func(c echo.Context) error {
		clients, err := self.logRepository.GetClients()

		if err != nil {
			return err
		}

		return c.JSON(200, clients)
	})

	handlers.NewLogsRouter(self.logRepository, api)

	api.GET("/", handlers.HomePage(self.logRepository))

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}
