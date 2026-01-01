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
