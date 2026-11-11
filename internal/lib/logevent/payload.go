package logevent

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/rpc"
)

const (
	// MaxMessageBytes is the LOGS stream's per-message limit. JetStream counts
	// NATS headers as well as the payload.
	MaxMessageBytes = 1 << 20
	// MaxPayloadBytes leaves room for the publisher's Nats-Msg-Id header.
	MaxPayloadBytes = MaxMessageBytes - 1<<10
	TruncatedSuffix = " [truncated]"
)

// EncodePayload returns the JSON published for line, at most MaxPayloadBytes.
// JSON escaping can grow a message up to sixfold, so an oversized message keeps
// its leading whole runes and ends with TruncatedSuffix. Timestamp, sequence
// and instance are unchanged. Event identity must be derived from these bytes.
func EncodePayload(line rpc.LogLine) (payload []byte, truncated bool, err error) {
	payload, err = json.Marshal(line)
	if err != nil || len(payload) <= MaxPayloadBytes {
		return payload, false, err
	}
	message := line.Message
	line.Message = TruncatedSuffix
	frame, err := json.Marshal(line)
	if err != nil {
		return nil, false, err
	}
	room := MaxPayloadBytes - len(frame)
	if room < 0 {
		return nil, false, errors.New("log event fields exceed the payload limit")
	}
	cut := 0
	for cut < len(message) {
		r, size := utf8.DecodeRuneInString(message[cut:])
		cost := encodedRuneBytes(r, size)
		if cost > room {
			break
		}
		room -= cost
		cut += size
	}
	line.Message = message[:cut] + TruncatedSuffix
	payload, err = json.Marshal(line)
	if err != nil {
		return nil, false, err
	}
	if len(payload) > MaxPayloadBytes {
		return nil, false, errors.New("truncated log event exceeds the payload limit")
	}
	return payload, true, nil
}

// encodedRuneBytes is an upper bound of encoding/json's output for one decoded
// rune, with the default HTML escaping. The suffix starts with ASCII, so cutting
// at a decoded boundary cannot change how the kept prefix is encoded.
func encodedRuneBytes(r rune, size int) int {
	switch {
	case r == utf8.RuneError && size == 1:
		return len(`\ufffd`)
	case r == '\u2028' || r == '\u2029':
		return len(`\u2028`)
	case r >= utf8.RuneSelf:
		return size
	case r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t':
		return 2
	case r < 0x20 || r == '<' || r == '>' || r == '&':
		return len(`\u0000`)
	default:
		return 1
	}
}
