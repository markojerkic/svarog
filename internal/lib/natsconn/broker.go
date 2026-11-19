package natsconn

import "context"

// Broker owns the lifecycle of the NATS broker used by the server. The caller
// starts it before connecting and closes all connections before shutdown.
type Broker interface {
	Start(ctx context.Context) error
	Connect(opts ConnectOptions) (*NatsConnection, error)
	Shutdown(ctx context.Context) error
}

type ConnectOptions struct {
	Name                   string
	EnableJetStream        bool
	DisableReconnectBuffer bool // live events must not replay from a stale local buffer
}
