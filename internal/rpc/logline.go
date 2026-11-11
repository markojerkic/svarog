package rpc

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type LogLine struct {
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Sequence   int       `json:"sequence"`
	InstanceId string    `json:"instanceId"`
}

func (l *LogLine) Validate() error {
	if l.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if l.Sequence < 0 {
		return errors.New("sequence must be non-negative")
	}
	if err := ValidateInstanceID(l.InstanceId); err != nil {
		return err
	}
	if !utf8.ValidString(l.Message) || strings.ContainsRune(l.Message, 0) {
		return errors.New("message must be UTF-8 without NUL characters")
	}
	return nil
}

func ValidateInstanceID(instance string) error {
	if len(instance) > 1024 || !utf8.ValidString(instance) || strings.ContainsRune(instance, 0) {
		return errors.New("instance id must be UTF-8, at most 1024 bytes, without NUL characters")
	}
	return nil
}
