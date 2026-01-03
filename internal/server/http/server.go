package http

import (
	"context"
	"errors"
	"fmt"
	"os"

	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	customMiddleware "github.com/markojerkic/svarog/internal/server/http/middleware"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type HttpServer struct {
	logService            db.LogService
	sessionStore          sessions.Store
	authService           auth.AuthService
	filesService          files.FileService
	projectsService       projects.ProjectsService
	natsCredentialService *serverauth.NatsCredentialService
	watchHub              *websocket.WatchHub

	serverPort int
	echo       *echo.Echo
}

type HttpServerOptions struct {
	LogService            db.LogService
	SessionStore          sessions.Store
	AuthService           auth.AuthService
	FilesService          files.FileService
	ProjectsService       projects.ProjectsService
	NatsCredentialService *serverauth.NatsCredentialService
	WatchHub              *websocket.WatchHub

	ServerPort int
}

func (self *HttpServer) Start() error {
	e := echo.New()
	self.echo = e
	e.Validator = &Validator{validator: validator.New()}
	e.HTTPErrorHandler = customMiddleware.CustomHTTPErrorHandler

	sessionMiddleware := session.MiddlewareWithConfig(session.Config{
		Store: self.sessionStore,
	})

	privateApi := e.Group("",
		sessionMiddleware,
		customMiddleware.AuthContextMiddleware(self.authService),
		customMiddleware.RestPasswordMiddleware())
	publicApi := e.Group("", sessionMiddleware)
	adminApi := privateApi.Group("/admin", customMiddleware.RequiresRoleMiddleware(auth.ADMIN))

	handlers.NewHomeHandler(privateApi, self.projectsService)
	handlers.NewProjectsRouter(self.projectsService, *self.natsCredentialService, adminApi)
	handlers.NewAuthRouter(self.authService, privateApi, publicApi)
	handlers.NewLogsRouter(self.logService, privateApi)
	handlers.NewWsConnectionRouter(self.watchHub, privateApi)

	e.Static("/assets", "internal/server/ui/assets")

	e.GET("/*", func(c echo.Context) error {
		// Serve requested file or fallback to index.html
		requestedFile := fmt.Sprintf("public/%s", c.Request().URL.Path)

		if _, err := os.Stat(requestedFile); errors.Is(err, os.ErrNotExist) {
			slog.Error("File not found", "file", requestedFile)
			return c.File("public/index.html")
		}

		return c.File(requestedFile)
	})

	serverAddr := fmt.Sprintf(":%d", self.serverPort)
	return e.Start(serverAddr)
}

func (self *HttpServer) Shutdown(ctx context.Context) error {
	if self.echo != nil {
		return self.echo.Shutdown(ctx)
	}
	return nil
}

func NewServer(options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logService:            options.LogService,
		sessionStore:          options.SessionStore,
		serverPort:            options.ServerPort,
		authService:           options.AuthService,
		filesService:          options.FilesService,
		projectsService:       options.ProjectsService,
		natsCredentialService: options.NatsCredentialService,
		watchHub:              options.WatchHub,
	}

	return server
}
