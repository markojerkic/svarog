package util

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

// SetupLogger configures the local console. Even if slog later forwards server
// logs, ingest and live-delivery diagnostics use charmbracelet directly so a
// failed delivery cannot produce another delivery.
// Call this once at startup.
func SetupLogger(level log.Level) {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level:           level,
		ReportCaller:    true,
		ReportTimestamp: true,
	})
	log.SetDefault(logger)
	slog.SetDefault(slog.New(logger))
}

// ParseLogLevel reads LOG_LEVEL. Empty means info. Accepted values are
// debug, info, warn, and error.
func ParseLogLevel(value string) (log.Level, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return log.InfoLevel, nil
	}

	level, err := log.ParseLevel(value)
	if err != nil || level == log.FatalLevel {
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: use debug, info, warn, or error", value)
	}
	return level, nil
}
