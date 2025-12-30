package config

import (
	"fmt"
	"net/url"

	"github.com/charmbracelet/log"
)

type ClientConfig struct {
	Protocol   string
	ServerAddr string
	Topic      string
	Token      string
	Debug      bool
	connString string
}

func NewClientConfig(connString string) (ClientConfig, error) {
	url, err := url.Parse(connString)
	if err != nil {
		log.Fatal("Failed to parse connection string", "err", err)
	}

	config := ClientConfig{
		connString: connString,
		Protocol:   url.Scheme,
		ServerAddr: url.Host,
		Topic:      url.Path,
		Token:      url.Query().Get("token"),
		Debug:      url.Query().Get("debug") == "true",
	}

	if err := config.Validate(); err != nil {
		return config, err
	}

	return config, nil
}

func (c *ClientConfig) GetConnString() string {
	return c.connString
}

func (c ClientConfig) Validate() error {
	if c.Protocol != "svarog" {
		return fmt.Errorf("invalid protocol: %s, should be 'svarog'", c.Protocol)
	}
	if c.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}
	if c.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}

	return nil
}
