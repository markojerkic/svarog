// Package svaroglog experiments with intercepting standard Go logging.
package svaroglog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// Setup installs an experimental interceptor for global slog and log calls.
// It prints intercepted messages and forwards them to the standard text handler.
// Call it before creating application loggers with slog.With.
func Setup() {
	h := &interceptor{next: slog.NewTextHandler(os.Stdout, nil)}
	slog.SetDefault(slog.New(h))
	slog.SetLogLoggerLevel(slog.LevelInfo)
}

type interceptor struct {
	next slog.Handler
}

func (h *interceptor) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *interceptor) Handle(ctx context.Context, record slog.Record) error {
	fmt.Println("[svarog interceptor]", record.Level, record.Message)
	return h.next.Handle(ctx, record)
}

func (h *interceptor) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &interceptor{next: h.next.WithAttrs(attrs)}
}

func (h *interceptor) WithGroup(name string) slog.Handler {
	return &interceptor{next: h.next.WithGroup(name)}
}

var _ slog.Handler = (*interceptor)(nil)
