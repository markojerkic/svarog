package handlers

import (
	"log/slog"
	"net/http"
	"sync"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type WsRouter struct {
	wsHub        websocket.WatchHub[types.StoredLog]
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	pingPong     chan bool
	wsHub        websocket.WatchHub[types.StoredLog]
	subscription websocket.Subscription[types.StoredLog]
}

type WsMessageType string

const (
	NewLine                    WsMessageType = "newLine"
	AddSubscriptionInstance    WsMessageType = "addSubscriptionInstance"
	RemoveSubscriptionInstance WsMessageType = "removeSubscriptionInstance"
	Ping                       WsMessageType = "ping"
	Pong                       WsMessageType = "pong"
)

type WsMessage struct {
	Type WsMessageType `json:"type"`
	Data interface{}   `json:"data"`
}

func (self *WsConnection) closeSubscription() {
	self.wsHub.Unsubscribe(&self.subscription)
	self.wsConnection.Close()
}

func (self *WsConnection) readPipe(wsWaitGroup *sync.WaitGroup) {
	defer wsWaitGroup.Done()

	var message WsMessage
	for {
		err := self.wsConnection.ReadJSON(&message)
		if err != nil {
			slog.Error("Error reading WS message", slog.Any("error", err))
			return
		}

		switch message.Type {
		case AddSubscriptionInstance:
			instance, ok := message.Data.(string)
			if !ok {
				slog.Error("Instance id is not string")
				continue
			}
			self.subscription.AddInstance(instance)
		case RemoveSubscriptionInstance:
			instance, ok := message.Data.(string)
			if !ok {
				slog.Error("Instance id is not string")
				continue
			}
			self.subscription.RemoveInstance(instance)
		case Ping:
			self.pingPong <- true
		default:
			slog.Error("Unknown message type", slog.Any("error", message.Type))
		}

	}
}

func (self *WsConnection) writePipe(wsWaitGroup *sync.WaitGroup) {
	defer wsWaitGroup.Done()

	for {
		select {
		case storedLogLine := <-self.subscription.GetUpdates():

			logLine := LogLine{
				ID:             storedLogLine.ID.Hex(),
				Timestamp:      storedLogLine.Timestamp.UnixMilli(),
				Content:        storedLogLine.LogLine,
				SequenceNumber: storedLogLine.SequenceNumber,
				Client:         storedLogLine.Client,
			}

			message := WsMessage{
				Type: NewLine,
				Data: logLine,
			}

			err := self.wsConnection.WriteJSON(message)
			if err != nil {
				slog.Error("Error writing WS message", slog.Any("error", err))
				return
			}

		case <-self.pingPong:
			message := WsMessage{
				Type: Pong,
			}
			err := self.wsConnection.WriteJSON(message)
			if err != nil {
				slog.Error("Error writing WS message", slog.Any("error", err))
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

func (self *WsRouter) connectionHandler(c echo.Context) error {
	clientId := c.Param("clientId")

	slog.Debug("Request made for client", slog.String("clientId", clientId))
	subscription := self.wsHub.Subscribe(clientId)
	slog.Debug("Created subscription", slog.Any("subscription", subscription))

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	slog.Debug("New WS connection", slog.Any("clientId", clientId))

	wsConnection := &WsConnection{
		clientId:     clientId,
		wsConnection: conn,
		subscription: *subscription,
		wsHub:        self.wsHub,
		pingPong:     make(chan bool),
	}

	wsWaitGroup := &sync.WaitGroup{}
	wsWaitGroup.Add(2)

	go wsConnection.readPipe(wsWaitGroup)
	go wsConnection.writePipe(wsWaitGroup)

	go func() {
		// wait until read and write pipes are done and then close the subscription
		wsWaitGroup.Wait()
		wsConnection.closeSubscription()
		slog.Debug("WS connection closed", slog.Any("clientId", clientId))
		self.wsHub.Unsubscribe(subscription)
	}()

	return nil
}

func NewWsConnectionRouter(hub websocket.WatchHub[types.StoredLog], parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws")
	router := &WsRouter{
		wsHub:        hub,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:clientId", router.connectionHandler)
	slog.Info("Created WS connection router")

	return router
}
