package websocket

import (
	"fmt"
	"time"

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

func (w *WatchHub) Subscribe(projectId, clientId string) (*LogSubscription, error) {
	lines := newLogSubscription()
	sub, err := w.conn.Subscribe(fmt.Sprintf("ws.logs.%s.%s", projectId, clientId), func(msg *nats.Msg) {
		lines.enqueue(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	lines.attach(sub)
	if err := sub.SetPendingLimits(subscriptionMaxMessages, subscriptionMaxBytes); err != nil {
		lines.Close()
		return nil, err
	}
	subStatus := sub.StatusChanged(nats.SubscriptionSlowConsumer, nats.SubscriptionClosed)
	connStatus := w.conn.StatusChanged(nats.RECONNECTING, nats.CLOSED)
	go func() {
		defer w.conn.RemoveStatusListener(connStatus)
		select {
		case <-lines.done:
		case <-subStatus:
			lines.fail(ErrLiveResync)
		case <-connStatus:
			lines.fail(ErrLiveResync)
		}
	}()
	if err := w.conn.FlushTimeout(5 * time.Second); err != nil {
		lines.Close()
		return nil, err
	}
	return lines, nil
}

// Resync marks the current view incomplete; it never retries a committed DB insert.
func (w *WatchHub) Resync(projectId, clientId string) error {
	return w.SendLogLine(projectId, clientId, []byte(`<div hx-swap-oob="afterbegin:#log-scroll-container"><span data-log-resync></span></div>`))
}
