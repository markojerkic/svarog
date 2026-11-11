package logevent_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/stretchr/testify/require"
)

// The client reader truncates raw lines to this size before encoding.
const readerLineBytes = 512 * 1024

func TestEncodePayload(t *testing.T) {
	at := time.Date(2026, 11, 11, 10, 0, 0, 1, time.UTC)
	line := func(message string) rpc.LogLine {
		return rpc.LogLine{Message: message, Timestamp: at, Sequence: 42, InstanceId: "web <1> & č"}
	}

	t.Run("a fitting event keeps the unchanged wire format", func(t *testing.T) {
		event := line(strings.Repeat("a", readerLineBytes))
		payload, truncated, err := logevent.EncodePayload(event)
		require.NoError(t, err, "a maximum-size reader line is encodable")
		want, err := json.Marshal(event)
		require.NoError(t, err, "encode the event as old clients did")
		require.False(t, truncated, "a plain maximum-size reader line fits without truncation")
		require.Equal(t, want, payload, "fitting events keep the existing JSON so their identity and old servers are unchanged")
	})

	t.Run("escaping expansion is truncated at a rune boundary within the limit", func(t *testing.T) {
		pattern := "a<&>\x01\n\"\\čž€😀\u2028"
		original := strings.Repeat(pattern, readerLineBytes/len(pattern)+1)[:readerLineBytes]
		original = strings.ToValidUTF8(original, "")
		payload, truncated, err := logevent.EncodePayload(line(original))
		require.NoError(t, err, "an event whose JSON escaping exceeds the limit is still published")
		require.True(t, truncated, "the escaped reader line is reported as truncated")
		require.LessOrEqual(t, len(payload), logevent.MaxPayloadBytes, "the final encoded bytes fit the shared payload budget")
		require.Greater(t, len(payload), logevent.MaxPayloadBytes-len(`\u0000`), "truncation keeps as much of the message as the budget allows")

		var decoded rpc.LogLine
		require.NoError(t, json.Unmarshal(payload, &decoded), "the truncated payload is valid JSON for the server")
		require.NoError(t, logevent.Validate(decoded), "the truncated event passes server validation")
		require.True(t, utf8.ValidString(decoded.Message), "truncation never splits a UTF-8 rune")
		require.True(t, strings.HasSuffix(decoded.Message, logevent.TruncatedSuffix), "the stored message is clearly marked as truncated")
		kept := strings.TrimSuffix(decoded.Message, logevent.TruncatedSuffix)
		require.NotEmpty(t, kept, "truncation keeps the start of the message")
		require.True(t, strings.HasPrefix(original, kept), "truncation keeps an exact prefix of the original message")
		require.True(t, decoded.Timestamp.Equal(at), "truncation preserves the event timestamp including nanoseconds")
		require.Equal(t, 42, decoded.Sequence, "truncation preserves the event sequence")
		require.Equal(t, "web <1> & č", decoded.InstanceId, "truncation preserves the instance")
	})
}

func TestMessageKey(t *testing.T) {
	subject := "logs.project.client"
	event := rpc.LogLine{Message: "same text", Timestamp: time.Date(2026, 11, 11, 10, 0, 0, 1, time.UTC), Sequence: 42, InstanceId: "web"}
	payload, _, err := logevent.EncodePayload(event)
	require.NoError(t, err, "encode the original event")
	retried, _, err := logevent.EncodePayload(event)
	require.NoError(t, err, "encode the same event again")
	require.Equal(t, logevent.MessageKey(subject, payload), logevent.MessageKey(subject, retried), "re-encoding a retried event keeps its identity")
	require.NotEqual(t, logevent.MessageKey(subject, payload), logevent.MessageKey("logs.project.other-client", payload), "identical payloads from different clients must not collide in the shared stream")
	event.Timestamp = event.Timestamp.Add(time.Nanosecond)
	next, _, err := logevent.EncodePayload(event)
	require.NoError(t, err, "encode the next event")
	require.NotEqual(t, logevent.MessageKey(subject, payload), logevent.MessageKey(subject, next), "event identity preserves nanoseconds before PostgreSQL timestamp rounding")
}
