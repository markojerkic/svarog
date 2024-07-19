package handlers

import (
	"log/slog"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/db"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type WsRouter struct {
	wsHub        websocket.WatchHub[db.StoredLog]
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	subscription websocket.Subscription[db.StoredLog]
}

type WsMessageType string

const (
	NewLine                    WsMessageType = "newLine"
	AddSubscriptionInstance    WsMessageType = "addSubscriptionInstance"
	RemoveSubscriptionInstance WsMessageType = "removeSubscriptionInstance"
)

type WsMessage struct {
	Type WsMessageType
	Data interface{}
}

func (self *WsConnection) readPipe() {
	defer func() {
		self.wsConnection.Close()
		self.subscription.Close()
	}()

	var message WsMessage
	for {
		err := self.wsConnection.ReadJSON(&message)
		if err != nil {
			slog.Error("Error reading WS message", err)
			continue
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
		default:
			slog.Error("Unknown message type", message.Type)
		}

	}
}

func (self *WsConnection) writePipe() {
	defer self.wsConnection.Close()
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

			err := self.wsConnection.WriteJSON(logLine)
			if err != nil {
				slog.Error("Error writing WS message", err)
				return
			}
		}
	}
}

var wsUpgrader = gorillaWs.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (self *WsRouter) connectionHandler(c echo.Context) error {
	clientId := c.Param("clientId")

	subscription := self.wsHub.Subscribe(clientId)

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	wsConnection := &WsConnection{
		clientId:     clientId,
		wsConnection: conn,
		subscription: *subscription,
	}

	go wsConnection.readPipe()
	go wsConnection.writePipe()

	return nil
}

func NewWsConnectionRouter(hub websocket.WatchHub[db.StoredLog], parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws")
	router := &WsRouter{
		wsHub:        hub,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:clientId", router.connectionHandler)

	return router
}
