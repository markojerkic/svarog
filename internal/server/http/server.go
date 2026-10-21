package http

import (
	"context"
	"fmt"
	stdhttp "net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
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
	projectsService       projects.ProjectsService
	natsCredentialService *serverauth.NatsCredentialService
	watchHub              *websocket.WatchHub

	serverPort int
	server     *stdhttp.Server
}

type HttpServerOptions struct {
	LogService            db.LogService
	SessionStore          sessions.Store
	AuthService           auth.AuthService
	ProjectsService       projects.ProjectsService
	NatsCredentialService *serverauth.NatsCredentialService
	WatchHub              *websocket.WatchHub

	ServerPort int
}

func (self *HttpServer) Start() error {
	// v5 catch-all 404 routes would let the two "" groups overwrite each other.
	e := echo.NewWithConfig(echo.Config{NoGroupAutoRegister404Routes: true})
	e.Use(customMiddleware.LanguageMiddleware)
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

	handlers.NewLanguageRouter(publicApi)
	handlers.NewHomeHandler(privateApi, self.projectsService)
	handlers.NewProjectsRouter(self.projectsService, *self.natsCredentialService, adminApi)
	handlers.NewLogsRouter(self.logService, self.authService, self.projectsService, privateApi)
	handlers.NewAuthRouter(self.authService, self.projectsService, adminApi, privateApi, publicApi)
	handlers.NewWsConnectionRouter(self.watchHub, self.authService, self.projectsService, privateApi)

	e.Static("/assets", "internal/server/ui/assets")

	self.server = &stdhttp.Server{
		Addr:    fmt.Sprintf(":%d", self.serverPort),
		Handler: e,
	}
	return self.server.ListenAndServe()
}

func (self *HttpServer) Shutdown(ctx context.Context) error {
	if self.server != nil {
		return self.server.Shutdown(ctx)
	}
	return nil
}

func NewServer(options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logService:            options.LogService,
		sessionStore:          options.SessionStore,
		serverPort:            options.ServerPort,
		authService:           options.AuthService,
		projectsService:       options.ProjectsService,
		natsCredentialService: options.NatsCredentialService,
		watchHub:              options.WatchHub,
	}

	return server
}
