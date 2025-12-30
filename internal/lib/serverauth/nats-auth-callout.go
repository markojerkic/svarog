package serverauth

import (
	"errors"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type NatsAuthCalloutHandler struct {
	natsIssuerKeyPair nkeys.KeyPair
	natsAuthUser      string
	natsAuthPassword  string
	natsAddr          string
}

func NewNatsAuthCalloutHandler() *NatsAuthCalloutHandler {
	issuerSeed := os.Getenv("NATS_ISSUER_SEED")
	if issuerSeed == "" {
		panic("NATS_ISSUER_SEED is not set")
	}

	issuerKp, err := nkeys.FromSeed([]byte(issuerSeed))
	if err != nil {
		panic(err)
	}

	return &NatsAuthCalloutHandler{
		natsIssuerKeyPair: issuerKp,
		natsAuthUser:      os.Getenv("NATS_SYSTEM_USER"),
		natsAuthPassword:  os.Getenv("NATS_SYSTEM_PASSWORD"),
		natsAddr:          os.Getenv("NATS_ADDR"),
	}
}

func (n *NatsAuthCalloutHandler) run() error {
	nc, err := nats.Connect(n.natsAddr,
		nats.UserInfo(n.natsAuthUser, n.natsAuthPassword),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return errors.Join(errors.New("nats connect failed"), err)
	}

	_, err = nc.Subscribe("$SYS.REQ.USER.AUTH", func(msg *nats.Msg) {
		log.Debug("nats auth callout", "msg", msg)
	})

	return err
}
