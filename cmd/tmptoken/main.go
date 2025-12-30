package main

import (
	"flag"
	"fmt"

	"github.com/markojerkic/svarog/internal/lib/serverauth"

	dotenv "github.com/joho/godotenv"
)

func main() {
	err := dotenv.Load()
	if err != nil {
		panic(err)
	}

	authhandler := serverauth.NewNatsAuthCalloutHandler()

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
