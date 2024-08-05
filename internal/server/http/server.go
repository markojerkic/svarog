package http

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type HttpServer struct {
	logRepository db.LogRepository

	allowedOrigins []string
	serverPort     int
}

type HttpServerOptions struct {
	AllowedOrigins []string
	ServerPort     int
}

func (self *HttpServer) Start() {
	e := echo.New()

	api := e.Group("/api/v1")

	if len(self.allowedOrigins) > 0 {
		api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: self.allowedOrigins,
		}))
	}

	api.GET("/clients", func(c echo.Context) error {
		clients, err := self.logRepository.GetClients()

		if err != nil {
			return err
		}

		return c.JSON(200, clients)
	})

	handlers.NewLogsRouter(self.logRepository, api)
	handlers.NewWsConnectionRouter(websocket.LogsHub, api)

	e.GET("/*", func(c echo.Context) error {
		// Serve requested file or fallback to index.html
		requestedFile := fmt.Sprintf("public/%s", c.Request().URL.Path)

		if _, err := os.Stat(requestedFile); errors.Is(err, os.ErrNotExist) {
			slog.Error("File not found", slog.String("file", requestedFile))
			return c.File("public/index.html")
		}

		return c.File(requestedFile)
	})

	serverAddr := fmt.Sprintf(":%d", self.serverPort)
	e.Logger.Fatal(e.Start(serverAddr))
}

func NewServer(logRepository db.LogRepository, options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logRepository:  logRepository,
		allowedOrigins: options.AllowedOrigins,
		serverPort:     options.ServerPort,
	}

	return server
}
