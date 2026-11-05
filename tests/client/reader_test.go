package reader_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/stretchr/testify/require"
)

func TestReaderContinuesAfterLongLine(t *testing.T) {
	t.Run("keeps a line past the old scanner limit", func(t *testing.T) {
		long := strings.Repeat("a", 512*1024)
		lines := readLines(long + "\nafter\n")

		require.Equal(t, []string{long, "after"}, lines, "a line up to 512KiB must be kept whole, and reading must continue")
	})

	t.Run("truncates an oversized line and keeps reading", func(t *testing.T) {
		lines := readLines(strings.Repeat("b", 2<<20) + "\nafter\n")
		truncated := strings.Repeat("b", 512*1024-len(" [truncated]")) + " [truncated]"

		require.Equal(t, []string{truncated, "after"}, lines, "an oversized line must be capped at 512KiB including its marker, and reading must continue")
	})
}

func readLines(input string) []string {
	out := make(chan *rpc.LogLine, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	reader.NewReader(strings.NewReader(input), out, "web").Run(context.Background(), &wg)
	close(out)

	var lines []string
	for line := range out {
		lines = append(lines, line.Message)
	}
	return lines
}
