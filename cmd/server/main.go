package main

import (
	"os"

	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/server/app"
)

func main() {
	cfg := app.LoadEnv[app.Config]()
	brokerCfg := app.LoadEnv[natsconn.ExternalConfig]()
	os.Exit(app.Main(cfg, app.WithBroker(natsconn.NewExternal(brokerCfg))))
}
