package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/lib/svaroglog"
)

func run() (result error) {
	connectionString := os.Getenv("SVAROG_CONN_STRING")
	if connectionString == "" {
		return errors.New("set SVAROG_CONN_STRING to a generated Svarog connection URL")
	}
	client, err := svarogclient.New(svarogclient.Options{
		ConnectionString: connectionString,
		InstanceID:       os.Getenv("SVAROG_INSTANCE_ID"),
	})
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, client.Shutdown(ctx))
	}()

	svaroglog.Setup(client, svaroglog.Options{
		Level:   slog.LevelDebug,
		Console: slog.NewTextHandler(os.Stdout, nil),
	})

	slog.Debug("Only sent to the client; console starts at INFO")
	slog.Info("Hello from slog", "order_id", 123)
	slog.With("component", "demo").WithGroup("request").Warn("Request failed", "id", 42)
	log.Printf("Hello from standard log: port=%d", 1323)

	return client.Enqueue(svarogclient.Entry{
		Message:   "Plain entry without slog",
		Timestamp: time.Now(),
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
