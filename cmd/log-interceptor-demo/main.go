package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/lib/svaroglog"
)

func main() {
	client := svarogclient.New(svarogclient.Options{
		ConnectionString: "svarog://localhost:4222/logs.<project>.<client>?token=<token>",
		InstanceID:       "demo",
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Shutdown(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "Client shutdown failed:", err)
		}
	}()

	svaroglog.Setup(client, svaroglog.Options{
		Level:   slog.LevelDebug,
		Console: slog.NewTextHandler(os.Stdout, nil),
	})

	slog.Debug("Only sent to the client; console starts at INFO")
	slog.Info("Hello from slog", "order_id", 123)
	slog.With("component", "demo").WithGroup("request").Warn("Request failed", "id", 42)
	log.Printf("Hello from standard log: port=%d", 1323)

	if err := client.Enqueue(svarogclient.Entry{
		Message:   "Plain entry without slog",
		Timestamp: time.Now(),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "Entry rejected:", err)
	}
}
