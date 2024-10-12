package http

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

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
	baseHref       string
}

type HttpServerOptions struct {
	AllowedOrigins []string
	ServerPort     int
	BaseHref       string
}

func (self *HttpServer) Start() {
	e := echo.New()

	var api *echo.Group
	if self.baseHref != "" {
		baseHref := fmt.Sprintf("%s/api/v1", self.baseHref)
		slog.Info("Base href set", slog.String("baseHref", baseHref))
		api = e.Group(baseHref)
	} else {
		api = e.Group("/api/v1")
	}

	err := self.prepareIndexHtml()
	if err != nil {
		slog.Error("Failed to prepare index.html", slog.Any("error", err))
	}

	if len(self.allowedOrigins) > 0 {
		api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: self.allowedOrigins,
		}))
	}

	api.GET("/clients", func(c echo.Context) error {
		clients, err := self.logRepository.GetClients(c.Request().Context())

		if err != nil {
			return err
		}

		return c.JSON(200, clients)
	})

	handlers.NewLogsRouter(self.logRepository, api)
	handlers.NewWsConnectionRouter(websocket.LogsHub, api)

	e.GET("/*", func(c echo.Context) error {
		requestedPath := c.Request().URL.Path
		// Strip path of baseHref
		if self.baseHref != "" && strings.HasPrefix(requestedPath, self.baseHref) {
			requestedPath = requestedPath[len(self.baseHref):]
		}
		// Serve requested file or fallback to index.html
		requestedFile := fmt.Sprintf("public/%s", requestedPath)

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
		baseHref:       options.BaseHref,
	}

	return server
}
