package archive_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/stretchr/testify/require"
)

func TestDiskStoreFailedWritesNeverPublishContent(t *testing.T) {
	for _, scenario := range []string{"producer error", "cancellation"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			store := archive.NewDiskStore(dir)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := errors.New("producer failed")
			if scenario == "cancellation" {
				failure = context.Canceled
			}
			const key = "client/archive.zip"
			_, err := store.Put(ctx, key, func(output io.Writer) error {
				_, err := io.WriteString(output, "partial bytes")
				require.NoError(t, err, "write some bytes before the failure")
				if scenario == "cancellation" {
					cancel()
					return nil
				}
				return failure
			})
			require.ErrorIs(t, err, failure, "failed writes preserve the original error")
			_, err = store.Open(t.Context(), key, int64(len("partial bytes")))
			require.Error(t, err, "failed writes never publish completed content")
		})
	}
}

func TestDiskStoreConfinesWritesAndDeletes(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.zip")
	require.NoError(t, os.WriteFile(outside, []byte("protected"), 0o600), "prepare content outside the storage root")
	require.NoError(t, os.Symlink(filepath.Dir(outside), filepath.Join(dir, "escape")), "prepare a directory symlink escaping the storage root")
	store := archive.NewDiskStore(dir)
	for _, key := range []string{outside, "../outside.zip", "escape/outside.zip"} {
		t.Run(key, func(t *testing.T) {
			_, err := store.Put(t.Context(), key, func(output io.Writer) error {
				_, err := io.WriteString(output, "overwrite")
				return err
			})
			require.Error(t, err, "unsafe keys cannot write outside the archive root")
			require.Error(t, store.Delete(t.Context(), key), "unsafe keys cannot remove files outside the archive root")
		})
	}
	body, err := os.ReadFile(outside)
	require.NoError(t, err, "the outside file remains present after all storage operations")
	require.Equal(t, "protected", string(body), "storage operations never modify outside content")
}
