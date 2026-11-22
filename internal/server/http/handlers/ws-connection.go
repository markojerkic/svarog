package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/google/uuid"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

type WsRouter struct {
	liveLogs     *live.Logs
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	lines        *live.Session
	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
}

const (
	wsWriteTimeout = 10 * time.Second
	wsPongTimeout  = 60 * time.Second
	wsPingInterval = 30 * time.Second
	// A new socket alone cannot repair this view. Unlike 1013 this is not retryable.
	wsCloseRefreshRequired = 4001
	wsCloseViewerQueueFull = 4002
)

func (self *WsConnection) closeSubscription() {
	self.closeOnce.Do(func() {
		self.cancel()
		self.lines.Close()
		self.wsConnection.Close()
	})
}

func (self *WsConnection) writePipe() {
	defer self.closeSubscription()
	for {
		renderedLogLine, err := self.lines.Next(self.ctx)
		if err != nil {
			if errors.Is(err, live.ErrLiveResync) {
				code, reason := wsCloseRefreshRequired, "live view requires refresh"
				if errors.Is(err, live.ErrViewerQueueFull) {
					code, reason = wsCloseViewerQueueFull, "live viewer queue full"
				}
				self.wsConnection.WriteControl(gorillaWs.CloseMessage,
					gorillaWs.FormatCloseMessage(code, reason),
					time.Now().Add(wsWriteTimeout))
			}
			return
		}
		self.wsConnection.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
		err = self.wsConnection.WriteMessage(gorillaWs.TextMessage, renderedLogLine)
		if err != nil {
			slog.Debug("Could not write live log batch", "error", err, "clientId", self.clientId)
			return
		}
	}
}

func (self *WsConnection) readPipe() {
	defer self.closeSubscription()
	self.wsConnection.SetReadLimit(1024)
	self.wsConnection.SetReadDeadline(time.Now().Add(wsPongTimeout))
	self.wsConnection.SetPongHandler(func(string) error {
		return self.wsConnection.SetReadDeadline(time.Now().Add(wsPongTimeout))
	})
	for {
		_, _, err := self.wsConnection.ReadMessage()
		if err != nil {
			slog.Debug("WebSocket read error (client likely disconnected)", "error", err, "clientId", self.clientId)
			return
		}
	}
}

func (self *WsConnection) heartbeat() {
	defer self.closeSubscription()
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-self.ctx.Done():
			return
		case <-ticker.C:
			// Gorilla permits WriteControl concurrently with the data writer.
			if err := self.wsConnection.WriteControl(gorillaWs.PingMessage, nil, time.Now().Add(wsWriteTimeout)); err != nil {
				return
			}
		}
	}
}

var wsUpgrader = gorillaWs.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (self *WsRouter) connectionHandler(c *echo.Context) error {
	clientId := c.Param("clientId")
	projectId := c.Param("projectId")
	selector, err := live.NewSelector(c.QueryParams()["instance"])
	if err != nil {
		return c.String(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_instances"))
	}
	projectID, projectErr := uuid.Parse(projectId)
	clientID, clientErr := uuid.Parse(clientId)
	if projectErr != nil || clientErr != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if !gorillaWs.IsWebSocketUpgrade(c.Request()) {
		return c.NoContent(http.StatusBadRequest)
	}
	// Only report a successful handshake once the subscription is ready.
	// A broker outage must not reset the browser's reconnect backoff on every attempt.
	subscription, err := self.liveLogs.Open(c.Request().Context(), logstore.Scope{ProjectID: projectID, ClientID: clientID}, selector)
	if err != nil {
		slog.Error("Could not subscribe to live logs", "error", err)
		return c.NoContent(http.StatusServiceUnavailable)
	}
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		subscription.Close()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())

	wsConnection := &WsConnection{
		clientId:     clientId,
		wsConnection: conn,
		lines:        subscription,
		ctx:          ctx,
		cancel:       cancel,
	}

	go wsConnection.writePipe()
	go wsConnection.readPipe()
	go wsConnection.heartbeat()

	return nil
}

func NewWsConnectionRouter(liveLogs *live.Logs, authService middleware.ProjectAccessChecker, projectsService middleware.ClientOwnershipChecker, parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws", middleware.RequiresProjectAccessMiddleware(authService, projectsService))
	router := &WsRouter{
		liveLogs:     liveLogs,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:projectId/:clientId", router.connectionHandler)
	slog.Info("Created WS connection router")

	return router
}
