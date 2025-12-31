package http

import (
	"errors"
	"fmt"
	"os"

	"log/slog"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	logService         db.LogService
	sessionStore       sessions.Store
	authService        auth.AuthService
	certificateService serverauth.CertificateService
	filesService       files.FileService
	projectsService    projects.ProjectsService

	allowedOrigins []string
	serverPort     int
}

type HttpServerOptions struct {
	LogService         db.LogService
	SessionStore       sessions.Store
	AuthService        auth.AuthService
	CertificateService serverauth.CertificateService
	FilesService       files.FileService
	ProjectsService    projects.ProjectsService

	AllowedOrigins []string
	ServerPort     int
}

func (self *HttpServer) Start() {
	e := echo.New()
	e.Validator = &Validator{validator: validator.New()}
	e.HTTPErrorHandler = customMiddleware.CustomHTTPErrorHandler

	sessionMiddleware := session.MiddlewareWithConfig(session.Config{
		Store: self.sessionStore,
	})
	corsMiddleware := middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     self.allowedOrigins,
		AllowCredentials: true,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods:     []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	})

	privateApi := e.Group("",
		sessionMiddleware,
		customMiddleware.AuthContextMiddleware(self.authService),
		customMiddleware.RestPasswordMiddleware())
	publicApi := e.Group("", corsMiddleware, sessionMiddleware)
	adminApi := e.Group("/admin", corsMiddleware, sessionMiddleware, customMiddleware.AuthContextMiddleware(self.authService), customMiddleware.RequiresRoleMiddleware(auth.ADMIN))

	handlers.NewHomeHandler(privateApi)
	handlers.NewProjectsRouter(self.projectsService, self.certificateService, adminApi)
	handlers.NewAuthRouter(self.authService, privateApi, publicApi)
	handlers.NewCertificateRouter(self.certificateService, self.filesService, privateApi)
	handlers.NewLogsRouter(self.logService, privateApi)
	handlers.NewWsConnectionRouter(websocket.LogsHub, privateApi)

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
	e.Logger.Fatal(e.Start(serverAddr))
}

func NewServer(options HttpServerOptions) *HttpServer {
	server := &HttpServer{
		logService:         options.LogService,
		sessionStore:       options.SessionStore,
		allowedOrigins:     options.AllowedOrigins,
		serverPort:         options.ServerPort,
		authService:        options.AuthService,
		certificateService: options.CertificateService,
		filesService:       options.FilesService,
		projectsService:    options.ProjectsService,
	}

	return server
}
