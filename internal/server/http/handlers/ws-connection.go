package handlers

import (
	"net/http"
	"sync"

	"log/slog"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/nats-io/nats.go"
)

type WsRouter struct {
	wsHub        *websocket.WatchHub
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	wsHub        *websocket.WatchHub
	lines        chan []byte
	natsSub      *nats.Subscription
}

type WsMessageType string

const (
	NewLine      WsMessageType = "newLine"
	SetInstances WsMessageType = "setInstances"
	Ping         WsMessageType = "ping"
	Pong         WsMessageType = "pong"
)

func (self *WsConnection) closeSubscription() {
	self.natsSub.Unsubscribe()
	self.wsConnection.Close()
}

func (self *WsConnection) writePipe(wsWaitGroup *sync.WaitGroup) {
	defer wsWaitGroup.Done()
	for renderedLogLine := range self.lines {
		err := self.wsConnection.WriteMessage(gorillaWs.TextMessage, renderedLogLine)
		if err != nil {
			slog.Error("Error writing WS message", "error", err)
			return
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

func (self *WsRouter) connectionHandler(c echo.Context) error {
	clientId := c.Param("clientId")
	projectId := c.Param("projectId")

	lines := make(chan []byte, 100)
	subscription, err := self.wsHub.Subscribe(projectId, clientId, lines)
	if err != nil {
		return err
	}

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	slog.Debug("New WS connection", "clientId", clientId)

	wsConnection := &WsConnection{
		clientId:     clientId,
		wsConnection: conn,
		wsHub:        self.wsHub,
		lines:        lines,
		natsSub:      subscription,
	}

	wsWaitGroup := &sync.WaitGroup{}
	wsWaitGroup.Add(2)

	go wsConnection.writePipe(wsWaitGroup)

	go func() {
		// wait until read and write pipes are done and then close the subscription
		wsWaitGroup.Wait()
		wsConnection.closeSubscription()
		slog.Debug("WS connection closed", "clientId", clientId)
		wsConnection.closeSubscription()
	}()

	return nil
}

func NewWsConnectionRouter(hub *websocket.WatchHub, parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws")
	router := &WsRouter{
		wsHub:        hub,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:projectId/:clientId", router.connectionHandler)
	slog.Info("Created WS connection router")

	return router
}
