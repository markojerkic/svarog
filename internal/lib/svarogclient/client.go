package svarogclient

import (
	"context"
	"fmt"
	"time"
)

type Entry struct {
	Message   string
	Timestamp time.Time
}

type Options struct {
	ConnectionString string
	InstanceID       string
}

type Client struct {
	options Options
}

func New(options Options) *Client {
	return &Client{options: options}
}

func (c *Client) Enqueue(entry Entry) error {
	_, err := fmt.Println("[svarog client stub]", c.options.InstanceID,
		entry.Timestamp.Format(time.RFC3339Nano), entry.Message)
	return err
}

func (c *Client) Shutdown(_ context.Context) error {
	return nil
}
