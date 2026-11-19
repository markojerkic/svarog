package main

import (
	"os"

	"github.com/markojerkic/svarog/internal/lib/natsconn/embedded"
	"github.com/markojerkic/svarog/internal/server/app"
)

func main() {
	cfg := app.LoadEnv[app.Config]()
	brokerCfg := app.LoadEnv[embedded.Config]()
	brokerCfg.AccountSeed = cfg.NatsAccountSeed
	os.Exit(app.Main(cfg, app.WithBroker(embedded.New(brokerCfg))))
}
