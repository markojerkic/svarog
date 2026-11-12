package client_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/stretchr/testify/require"
)

func TestReaderContinuesAfterLongLine(t *testing.T) {
	t.Run("keeps a line past the old scanner limit", func(t *testing.T) {
		long := strings.Repeat("a", 512*1024)
		lines := readLines(t, long+"\nafter\n")

		require.Equal(t, []string{long, "after"}, lines, "a line up to 512KiB must be kept whole, and reading must continue")
	})

	t.Run("truncates an oversized line and keeps reading", func(t *testing.T) {
		lines := readLines(t, strings.Repeat("b", 2<<20)+"\nafter\n")
		truncated := strings.Repeat("b", 512*1024-len(" [truncated]")) + " [truncated]"

		require.Equal(t, []string{truncated, "after"}, lines, "an oversized line must be capped at 512KiB including its marker, and reading must continue")
	})

	t.Run("truncation preserves valid UTF-8", func(t *testing.T) {
		lines := readLines(t, strings.Repeat("😀", 1<<18)+"\nafter\n")
		require.Len(t, lines, 2, "reading must continue after a truncated Unicode message")
		require.True(t, utf8.ValidString(lines[0]), "truncation must not turn a valid log into an invalid payload rejected by the shared client")
		require.LessOrEqual(t, len(lines[0]), 512*1024, "the truncated message and marker must stay within the reader limit")
		require.Equal(t, "after", lines[1], "the next log line must not be lost")
	})
}

func readLines(t *testing.T, input string) []string {
	t.Helper()
	var lines []string
	err := reader.NewReader(strings.NewReader(input), func(entry svarogclient.Entry) error {
		lines = append(lines, entry.Message)
		return nil
	}).Run(context.Background())
	require.NoError(t, err, "the stdin reader must successfully submit each bounded line")
	return lines
}
