package rpc_test

import (
	"testing"

	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/stretchr/testify/require"
)

func TestMessageKey(t *testing.T) {
	subject := "logs.project.client"
	payload := []byte(`{"message":"same text","timestamp":"2026-11-11T10:00:00.000000001Z","sequence":42,"instanceId":"web"}`)
	key := rpc.MessageKey(subject, payload)
	require.Equal(t, key, rpc.MessageKey(subject, append([]byte(nil), payload...)), "a retried original payload keeps the same identity")
	require.NotEqual(t, key, rpc.MessageKey("logs.project.other-client", payload), "identical payloads from different clients must not collide in the shared stream")
	next := []byte(`{"message":"same text","timestamp":"2026-11-11T10:00:00.000000002Z","sequence":42,"instanceId":"web"}`)
	require.NotEqual(t, key, rpc.MessageKey(subject, next), "event identity preserves nanoseconds before PostgreSQL timestamp rounding")
}
