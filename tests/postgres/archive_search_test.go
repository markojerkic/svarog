package postgres_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/tests/testutils"
)

func TestSearchArchivedRecords(t *testing.T) {
	ctx := t.Context()
	pool, service, scope, dir := newArchiveFixture(t, ctx, "ArchiveSearch")
	start := time.Date(2020, 3, 1, 12, 0, 0, 0, time.UTC)
	row := func(sec int, instance, text string) logstore.Row {
		return logstore.Row{Scope: logstore.Scope{ClientID: scope.ClientID}, OccurredAt: start.Add(time.Duration(sec) * time.Second), Instance: instance, Message: text}
	}
	const instance = "web /]\nč"
	const trace = "Timed OUT\r\n    at Store.Commit()\n\nretry failed"
	rows := []logstore.Row{row(-1, "worker", "outside"), row(0, instance, trace), row(300, "worker", "upper bound")}
	for i := 1; i <= 220; i++ {
		rows = append(rows, row(i, "worker", "request"))
	}
	build := func(rows []logstore.Row) uuid.UUID {
		_, err := testutils.InsertLogFixtures(ctx, pool, rows)
		require.NoError(t, err)
		result, err := service.Create(ctx, scope)
		require.NoError(t, err)
		require.NotNil(t, result.Archive)
		return result.Archive.ID
	}
	first := build(rows)
	build([]logstore.Row{row(4, "", "late arrival"), row(250, "worker", "newest")})
	query := archive.SearchQuery{From: start, To: start.Add(300 * time.Second)}
	result, err := service.Search(ctx, scope, query)
	require.NoError(t, err)
	require.Equal(t, 2, result.ArchivesScanned)
	require.Equal(t, 223, result.TotalMatches)
	require.Len(t, result.Hits, 200)
	require.Equal(t, "newest", result.Hits[0].Message)
	require.True(t, result.Hits[199].OccurredAt.Equal(start.Add(22*time.Second)))

	query.Text = "commit TIMED"
	result, err = service.Search(ctx, scope, query)
	require.NoError(t, err)
	require.Len(t, result.Hits, 1)
	require.Equal(t, trace, result.Hits[0].Message)
	require.Equal(t, instance, result.Hits[0].Instance)
	require.Equal(t, first, result.Hits[0].ArchiveID)

	_, err = service.Search(ctx, archive.Scope{ProjectID: uuid.New(), ClientID: scope.ClientID}, query)
	require.ErrorIs(t, err, archive.ErrClientNotFound)
	query.Text = "absent"
	result, err = service.Search(ctx, scope, query)
	require.NoError(t, err)
	require.Equal(t, 2, result.ArchivesScanned)
	require.Zero(t, result.TotalMatches)

	// A same-length modification passes the store's size check but not SHA-256.
	path := filepath.Join(dir, scope.ClientID.String(), first.String()+".zip")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	data[len(data)/2] ^= 1
	require.NoError(t, os.WriteFile(path, data, 0o600))
	result, err = service.Search(ctx, scope, query)
	require.ErrorIs(t, err, archive.ErrArchiveUnavailable)
	require.Equal(t, archive.SearchResult{}, result)
	query.From, query.To = start.Add(time.Hour), start.Add(2*time.Hour)
	result, err = service.Search(ctx, scope, query)
	require.NoError(t, err, "non-overlapping corrupt archives are not opened")
	require.Zero(t, result.ArchivesScanned)

	// The compressed-size budget must reject the request before trying to read
	// this unavailable object, even when the requested time range is tiny.
	_, err = pool.Exec(ctx, `UPDATE archives SET size_bytes = 1024::bigint * 1024 * 1024 WHERE id = $1`, first)
	require.NoError(t, err)
	_, err = service.Search(ctx, scope, archive.SearchQuery{From: start, To: start.Add(time.Second)})
	require.ErrorIs(t, err, archive.ErrSearchLimit)
}
