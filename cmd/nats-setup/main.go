package main

import (
	"fmt"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
)

func main() {
	setup, err := serverauth.GenerateNatsAuthSetup()
	if err != nil {
		panic(err)
	}

	fmt.Println(setup.PrintEnvVars())
}
