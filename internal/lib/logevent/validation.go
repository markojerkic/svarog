// Package logevent owns shared event validation and identity rules. The rpc
// package describes only the wire payload; clients and ingest share these rules.
package logevent

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/rpc"
)

const MaxInstanceIDBytes = 1024

func Validate(line rpc.LogLine) error {
	if line.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	if line.Sequence < 0 {
		return errors.New("sequence must be non-negative")
	}
	if err := ValidateInstanceID(line.InstanceId); err != nil {
		return err
	}
	if !utf8.ValidString(line.Message) || strings.ContainsRune(line.Message, 0) {
		return errors.New("message must be UTF-8 without NUL characters")
	}
	return nil
}

func ValidateInstanceID(instance string) error {
	if len(instance) > MaxInstanceIDBytes || !utf8.ValidString(instance) || strings.ContainsRune(instance, 0) {
		return errors.New("instance id must be UTF-8, at most 1024 bytes, without NUL characters")
	}
	return nil
}
