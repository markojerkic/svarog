package util

import (
	"log/slog"
	"os"

	"github.com/charmbracelet/log"
)

type LoggerOptions struct {
	Debug bool
}

// SetupLogger configures slog to use charmbracelet/log as the handler
// for pretty colored output. Call this once at startup.
func SetupLogger(opts ...LoggerOptions) {
	level := log.DebugLevel
	if len(opts) > 0 && !opts[0].Debug {
		level = log.ErrorLevel
	}

	handler := log.NewWithOptions(os.Stderr, log.Options{
		Level:           level,
		ReportCaller:    true,
		ReportTimestamp: true,
	})
	slog.SetDefault(slog.New(handler))
}
