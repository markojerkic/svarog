package handlers

import (
	"net/http"
	"sync"

	"github.com/charmbracelet/log"
	gorillaWs "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
)

type WsRouter struct {
	wsHub        websocket.WatchHub
	parentRouter *echo.Group
	api          *echo.Group
}

type WsConnection struct {
	clientId     string
	wsConnection *gorillaWs.Conn
	pingPong     chan bool
	wsHub        websocket.WatchHub
	subscription websocket.Subscription
}

type WsMessageType string

const (
	NewLine      WsMessageType = "newLine"
	SetInstances WsMessageType = "setInstances"
	Ping         WsMessageType = "ping"
	Pong         WsMessageType = "pong"
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
			log.Error("Error reading WS message", "error", err)
			return
		}

		switch message.Type {
		case SetInstances:
			instancesMap, ok := message.Data.([]interface{})
			if !ok {
				log.Error("Instances is a string array")
				continue
			}
			instances := make([]string, 0, len(instancesMap))
			for _, instance := range instancesMap {
				instances = append(instances, instance.(string))
			}
			log.Info("Setting instances", "instances", instances)
			self.subscription.SetInstances(instances)
		case Ping:
			self.pingPong <- true
		default:
			log.Error("Unknown message type", "error", message.Type)
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
				log.Error("Error writing WS message", "error", err)
				return
			}

		case <-self.pingPong:
			message := WsMessage{
				Type: Pong,
			}
			err := self.wsConnection.WriteJSON(message)
			if err != nil {
				log.Error("Error writing WS message", "error", err)
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

	log.Debug("Request made for client", "clientId", clientId)
	subscription := self.wsHub.Subscribe(clientId)
	log.Debug("Created subscription", "subscription", subscription)

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	log.Debug("New WS connection", "clientId", clientId)

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
		log.Debug("WS connection closed", "clientId", clientId)
		self.wsHub.Unsubscribe(subscription)
	}()

	return nil
}

func NewWsConnectionRouter(hub websocket.WatchHub, parentRouter *echo.Group) *WsRouter {
	api := parentRouter.Group("/ws")
	router := &WsRouter{
		wsHub:        hub,
		parentRouter: parentRouter,
		api:          api,
	}

	api.GET("/:clientId", router.connectionHandler)
	log.Info("Created WS connection router")

	return router
}
