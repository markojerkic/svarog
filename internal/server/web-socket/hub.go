package websocket

import (
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type WatchHub struct {
	conn *nats.Conn
}

func NewWatchHub(conn *nats.Conn) *WatchHub {
	return &WatchHub{
		conn: conn,
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
