package rpc

import (
	"errors"
	"time"
)

type LogLine struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Sequence  int       `json:"sequence"`
}

func (l *LogLine) Validate() error {
	if l.Message == "" {
		return errors.New("message is required")
	}
	if l.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if l.Sequence < 0 {
		return errors.New("sequence must be non-negative")
	}
	return nil
}
