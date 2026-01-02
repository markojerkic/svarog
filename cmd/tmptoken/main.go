package main

import (
	"encoding/base64"
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

	credService, err := serverauth.NewNatsCredentialService(os.Getenv("NATS_ACCOUNT_SEED"))
	if err != nil {
		panic(err)
	}

	// Generate credentials without expiry (nil = never expires)
	credsFile, err := credService.GenerateCredsFile("svarog-temp", *topic, nil)
	if err != nil {
		panic(err)
	}

	// Base64 encode for URL-safe transport
	encoded := base64.StdEncoding.EncodeToString([]byte(credsFile))
	fmt.Print(encoded)
}
