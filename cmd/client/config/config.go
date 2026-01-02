package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type ClientConfig struct {
	Protocol   string
	ServerAddr string
	Topic      string
	Creds      string
	Debug      bool
	connString string
}

func NewClientConfig(connString string) (ClientConfig, error) {
	url, err := url.Parse(connString)
	if err != nil {
		panic(fmt.Errorf("Failed to parse connection string: %w", err))
	}

	tokenB64 := url.Query().Get("token")
	var creds string
	if tokenB64 != "" {
		credsBytes, err := base64.StdEncoding.DecodeString(tokenB64)
		if err != nil {
			return ClientConfig{}, fmt.Errorf("failed to decode credentials: %w", err)
		}
		creds = string(credsBytes)
	}

	config := ClientConfig{
		connString: connString,
		Protocol:   url.Scheme,
		ServerAddr: url.Host,
		Topic:      strings.TrimPrefix(url.Path, "/"),
		Creds:      creds,
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

func (c *ClientConfig) GetNatsUrl() string {
	return fmt.Sprintf("nats://%s", c.ServerAddr)
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
	if c.Creds == "" {
		return fmt.Errorf("credentials are required (token param)")
	}

	return nil
}
