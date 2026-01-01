package websocket

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

type WatchHub struct {
	conn          *nats.Conn
	wsLogRenderer *WsLogLineRenderer
}

func NewWatchHub(conn *nats.Conn) *WatchHub {
	wsLogRenderer := NewWsLogLineRenderer()
	return &WatchHub{
		conn:          conn,
		wsLogRenderer: wsLogRenderer,
	}
}

func (w *WatchHub) SendLogLine(projectId, clientId string, line []byte) error {
	return w.conn.Publish(fmt.Sprintf("ws.logs.%s.%s", projectId, clientId), line)
}

func (w *WatchHub) Subscribe(projectId, clientId string, lines chan<- []byte) (*nats.Subscription, error) {
	return w.conn.Subscribe(fmt.Sprintf("ws.logs.%s.%s", projectId, clientId), func(msg *nats.Msg) {
		lines <- msg.Data
	})
}
