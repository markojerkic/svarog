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

	topic := flag.String("topic", "", "Topic to grant access to")
	flag.Parse()
	if *topic == "" {
		panic("Topic is required")
	}

	tokenService, err := serverauth.NewTokenService(os.Getenv("NATS_JWT_SECRET"))
	if err != nil {
		panic(err)
	}

	token, err := tokenService.GenerateToken("svarog-temp", *topic)
	if err != nil {
		panic(err)
	}

	fmt.Print(token)
}
