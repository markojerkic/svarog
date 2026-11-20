package http

import (
	"context"
	"fmt"
	stdhttp "net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	customMiddleware "github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/mcpserver"
)

type HttpServer struct {
	logReader               handlers.LogReader
	sessionStore            sessions.Store
	authService             *auth.PostgresAuthService
	projectsService         *projects.PostgresProjectsService
	archiver                archive.Archiver
	archiveSchedulerEnabled bool
	natsCredentialService   *serverauth.NatsCredentialService
	liveLogs                *live.Logs
	oauth                   *oauth.Service
	archiveLinks            *archivelink.Signer

	server *stdhttp.Server
}

type HttpServerOptions struct {
	LogReader               handlers.LogReader
	SessionStore            sessions.Store
	AuthService             *auth.PostgresAuthService
	ProjectsService         *projects.PostgresProjectsService
	Archiver                archive.Archiver
	ArchiveSchedulerEnabled bool
	NatsCredentialService   *serverauth.NatsCredentialService
	LiveLogs                *live.Logs
	// OAuth enables the MCP endpoint and its authorization server when non-nil.
	OAuth *oauth.Service
	// ArchiveLinks signs the archive download links issued by MCP tools.
	ArchiveLinks *archivelink.Signer

	ServerPort int
}

func (self *HttpServer) Start() error {
	// v5 catch-all 404 routes would let the two "" groups overwrite each other.
	e := echo.NewWithConfig(echo.Config{NoGroupAutoRegister404Routes: true})
	e.Use(customMiddleware.LanguageMiddleware)
	e.Validator = NewValidator()
	e.HTTPErrorHandler = customMiddleware.CustomHTTPErrorHandler

	sessionMiddleware := session.MiddlewareWithConfig(session.Config{
		Store: self.sessionStore,
	})

	privateApi := e.Group("",
		customMiddleware.NoStoreMiddleware,
		sessionMiddleware,
		customMiddleware.AuthContextMiddleware(self.authService),
		customMiddleware.RestPasswordMiddleware())
	publicApi := e.Group("", sessionMiddleware)
	adminApi := privateApi.Group("/admin", customMiddleware.RequiresRoleMiddleware(auth.ADMIN))

	handlers.NewLanguageRouter(publicApi)
	handlers.NewHomeHandler(privateApi, self.projectsService)
	handlers.NewProjectsRouter(self.projectsService, self.natsCredentialService, adminApi)
	handlers.NewLogsRouter(self.logReader, self.authService, self.projectsService, privateApi)
	handlers.NewArchiveRouter(self.archiver, self.archiveSchedulerEnabled, self.authService, self.projectsService, privateApi)
	handlers.NewAuthRouter(self.authService, self.projectsService, adminApi)
	handlers.RegisterLogout(self.authService, privateApi)
	handlers.RegisterPasswordReset(self.authService, privateApi)
	handlers.RegisterLogin(self.authService, publicApi)
	handlers.NewWsConnectionRouter(self.liveLogs, self.authService, self.projectsService, privateApi)
	if self.oauth != nil {
		self.registerMCP(e, privateApi)
	}

	e.Static("/assets", "internal/server/ui/assets", customMiddleware.AssetsCacheMiddleware)

	self.server.Handler = e
	return self.server.ListenAndServe()
}

// registerMCP serves the MCP endpoint outside the session middleware: it
// authenticates every request with an OAuth access token instead of a cookie.
func (self *HttpServer) registerMCP(e *echo.Echo, privateApi *echo.Group) {
	handlers.RegisterOAuth(self.oauth, e, privateApi)
	handlers.RegisterArchiveLinks(e, self.archiveLinks, self.authService, self.projectsService, self.archiver)
	resourceMetadata := echo.WrapHandler(mcpserver.NewResourceMetadataHandler(self.oauth.Issuer(), self.oauth.Resource()))
	e.GET(mcpserver.ResourceMetadataPath, resourceMetadata)
	e.OPTIONS(mcpserver.ResourceMetadataPath, resourceMetadata)
	e.Any(mcpserver.Path, echo.WrapHandler(mcpserver.NewHandler(mcpserver.Options{
		Issuer:       self.oauth.Issuer(),
		Resource:     self.oauth.Resource(),
		Tokens:       self.oauth,
		Users:        self.authService,
		Projects:     self.projectsService,
		Logs:         self.logReader,
		Archives:     self.archiver,
		ArchiveLinks: self.archiveLinks,
	})))
}

func (self *HttpServer) Shutdown(ctx context.Context) error {
	if self.server != nil {
		if err := self.server.Shutdown(ctx); err != nil {
			_ = self.server.Close()
			return err
		}
	}
	return nil
}

func NewServer(options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logReader:               options.LogReader,
		sessionStore:            options.SessionStore,
		server:                  &stdhttp.Server{Addr: fmt.Sprintf(":%d", options.ServerPort)},
		authService:             options.AuthService,
		projectsService:         options.ProjectsService,
		archiver:                options.Archiver,
		archiveSchedulerEnabled: options.ArchiveSchedulerEnabled,
		natsCredentialService:   options.NatsCredentialService,
		liveLogs:                options.LiveLogs,
		oauth:                   options.OAuth,
		archiveLinks:            options.ArchiveLinks,
	}

	return server
}
