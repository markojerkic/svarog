package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout); err != nil {
		slog.Error("Log generator failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, output io.Writer) error {
	interval, err := time.ParseDuration(envOr("DEMO_INTERVAL", "1s"))
	if err != nil || interval <= 0 {
		return fmt.Errorf("DEMO_INTERVAL must be a positive duration, for example 1s or 250ms")
	}

	path := envOr("DEMO_MESSAGES_FILE", "cmd/log-demo/messages.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read messages: %w", err)
	}
	var messages []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if message := strings.TrimSpace(line); message != "" {
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 {
		return fmt.Errorf("%s must contain at least one non-empty line", path)
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("app", envOr("DEMO_APP_NAME", "demo"))
	levels := []slog.Level{
		slog.LevelInfo, slog.LevelInfo, slog.LevelInfo,
		slog.LevelInfo, slog.LevelInfo, slog.LevelInfo,
		slog.LevelDebug, slog.LevelWarn, slog.LevelWarn, slog.LevelError,
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			logger.Log(ctx, levels[rand.IntN(len(levels))], messages[rand.IntN(len(messages))])
		}
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
