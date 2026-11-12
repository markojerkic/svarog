package svarogclient

import (
	"context"
	"net"
)

// Cancel both dialing and an in-progress NATS handshake when drain time expires.
type contextDialer struct {
	ctx context.Context
}

func (d *contextDialer) Dial(network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: connectTimeout}
	conn, err := dialer.DialContext(d.ctx, network, address)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(d.ctx, func() { _ = conn.Close() })
	return &contextConn{Conn: conn, stop: stop}, nil
}

type contextConn struct {
	net.Conn
	stop func() bool
}

func (c *contextConn) Close() error {
	c.stop()
	return c.Conn.Close()
}
