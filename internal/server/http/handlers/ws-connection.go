package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"log/slog"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type WsRouter struct {
	wsHub        *websocket.WatchHub
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	lines        *websocket.LogSubscription
	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
}

const (
	wsWriteTimeout = 10 * time.Second
	wsPongTimeout  = 60 * time.Second
	wsPingInterval = 30 * time.Second
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
			if errors.Is(err, websocket.ErrLiveResync) {
				self.wsConnection.WriteControl(gorillaWs.CloseMessage,
					gorillaWs.FormatCloseMessage(gorillaWs.CloseTryAgainLater, "live view requires refresh"),
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

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	subscription, err := self.wsHub.Subscribe(projectId, clientId)
	if err != nil {
		conn.Close()
		slog.Error("Could not subscribe to live logs", "error", err)
		return nil // the HTTP connection has already been upgraded
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

func NewWsConnectionRouter(hub *websocket.WatchHub, authService auth.AuthService, projectsService projects.ProjectsService, parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws", middleware.RequiresProjectAccessMiddleware(authService, projectsService))
	router := &WsRouter{
		wsHub:        hub,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:projectId/:clientId", router.connectionHandler)
	slog.Info("Created WS connection router")

	return router
}
