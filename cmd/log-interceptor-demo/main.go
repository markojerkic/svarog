package main

import (
	"log"
	"log/slog"

	"github.com/markojerkic/svarog/svaroglog"
)

func main() {
	svaroglog.Setup()

	slog.Info("Hello from slog", "order_id", 123)
	slog.With("component", "demo").WithGroup("request").Warn("Request failed", "id", 42)
	log.Printf("Hello from standard log: port=%d", 1323)
}
