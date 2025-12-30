package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/markojerkic/svarog/internal/lib/serverauth"

	dotenv "github.com/joho/godotenv"
)

func main() {
	err := dotenv.Load()
	if err != nil {
		panic(err)
	}

	authhandler, err := serverauth.NewNatsAuthCalloutHandler(serverauth.NatsAuthConfig{
		IssuerSeed:     os.Getenv("NATS_ISSUER_SEED"),
		JwtSecret:      os.Getenv("NATS_JWT_SECRET"),
		SystemUser:     os.Getenv("NATS_SYSTEM_USER"),
		SystemPassword: os.Getenv("NATS_SYSTEM_PASSWORD"),
		NatsAddr:       os.Getenv("NATS_ADDR"),
	})
	if err != nil {
		panic(err)
	}

	topic := flag.String("topic", "", "Topic to grant access to")
	flag.Parse()
	if *topic == "" {
		panic("Topic is required")
	}

	token, err := authhandler.GenerateToken("svarog-temp", *topic)
	if err != nil {
		panic(err)
	}

	fmt.Print(token)
}
