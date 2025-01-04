package http

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type HttpServer struct {
	logRepository db.LogRepository
	sessionStore  sessions.Store
	authService   auth.AuthService

	allowedOrigins []string
	serverPort     int
}

type HttpServerOptions struct {
	LogRepository  db.LogRepository
	SessionStore   sessions.Store
	AuthService    auth.AuthService
	AllowedOrigins []string
	ServerPort     int
}

func (self *HttpServer) Start() {
	e := echo.New()
	e.Validator = &Validator{validator: validator.New()}

	api := e.Group("/api/v1")
	api.Use(session.MiddlewareWithConfig(session.Config{
		Store: self.sessionStore,
	}))

	if len(self.allowedOrigins) > 0 {
		log.Info("Allowed origins", "origins", self.allowedOrigins)
		api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     self.allowedOrigins,
			AllowCredentials: true,
			AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
			AllowMethods:     []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}))
	} else {
		log.Warn("No allowed origins set, allowing all origins")
	}

	api.GET("/clients", func(c echo.Context) error {
		clients, err := self.logRepository.GetClients(c.Request().Context())
		session := c.Get("session")
		log.Info("session", "session", session)

		if err != nil {
			return err
		}

		return c.JSON(200, clients)
	})

	handlers.NewLogsRouter(self.logRepository, api)
	handlers.NewWsConnectionRouter(websocket.LogsHub, api)
	handlers.NewAuthRouter(self.authService, api)

	e.GET("/*", func(c echo.Context) error {
		// Serve requested file or fallback to index.html
		requestedFile := fmt.Sprintf("public/%s", c.Request().URL.Path)

		if _, err := os.Stat(requestedFile); errors.Is(err, os.ErrNotExist) {
			log.Error("File not found", "file", requestedFile)
			return c.File("public/index.html")
		}

		return c.File(requestedFile)
	})

	serverAddr := fmt.Sprintf(":%d", self.serverPort)
	e.Logger.Fatal(e.Start(serverAddr))
}

func NewServer(options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logRepository:  options.LogRepository,
		sessionStore:   options.SessionStore,
		allowedOrigins: options.AllowedOrigins,
		serverPort:     options.ServerPort,
		authService:    options.AuthService,
	}

	return server
}
