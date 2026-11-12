// Package svaroglog adapts standard Go logging to the shared client.
package svaroglog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
)

type Options struct {
	// Level is the minimum level sent to the client; nil means INFO.
	Level slog.Leveler
	// Console optionally receives logs too. Nil disables local console output.
	// Use an explicit handler, not the initial slog.Default().Handler().
	Console slog.Handler
}

// Setup installs the handler for global slog and log calls.
// The caller owns the client and must call its Shutdown method.
func Setup(client *svarogclient.Client, options Options) {
	slog.SetDefault(slog.New(NewHandler(client, options)))
	slog.SetLogLoggerLevel(slog.LevelInfo)
}

// NewHandler creates a handler without changing the global logger.
func NewHandler(client *svarogclient.Client, options Options) slog.Handler {
	output := &bytes.Buffer{}
	h := &interceptor{
		client:    client,
		output:    output,
		mu:        &sync.Mutex{},
		formatter: slog.NewTextHandler(output, &slog.HandlerOptions{Level: options.Level}),
	}
	if options.Console != nil {
		return slog.NewMultiHandler(options.Console, h)
	}
	return h
}

type interceptor struct {
	client    *svarogclient.Client
	output    *bytes.Buffer
	mu        *sync.Mutex
	formatter slog.Handler
}

func (h *interceptor) Enabled(ctx context.Context, level slog.Level) bool {
	return h.formatter.Enabled(ctx, level)
}

func (h *interceptor) Handle(ctx context.Context, record slog.Record) error {
	message, err := h.format(ctx, record)
	if err != nil {
		return err
	}
	return h.client.Enqueue(svarogclient.Entry{
		Message:   message,
		Timestamp: record.Time,
	})
}

func (h *interceptor) format(ctx context.Context, record slog.Record) (string, error) {
	// Derived handlers share the buffer, so they must also share this lock.
	h.mu.Lock()
	defer h.mu.Unlock()
	h.output.Reset()

	// Preserve the timestamp in Entry instead of duplicating it in Message.
	record.Time = time.Time{}
	if err := h.formatter.Handle(ctx, record); err != nil {
		return "", err
	}
	return strings.TrimSuffix(h.output.String(), "\n"), nil
}

func (h *interceptor) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.formatter = h.formatter.WithAttrs(attrs)
	return &clone
}

func (h *interceptor) WithGroup(name string) slog.Handler {
	clone := *h
	clone.formatter = h.formatter.WithGroup(name)
	return &clone
}

var _ slog.Handler = (*interceptor)(nil)
