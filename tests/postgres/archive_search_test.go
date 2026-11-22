package postgres_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	rows := []logstore.Row{row(-1, "previous", "outside"), row(0, instance, trace), row(300, "later", "upper bound")}
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
	require.NotEmpty(t, result.OlderCursor)
	next := query
	next.OlderCursor = result.OlderCursor
	last, err := service.Search(ctx, scope, next)
	require.NoError(t, err)
	require.Len(t, last.Hits, 23)
	require.Equal(t, result.TotalMatches, last.TotalMatches)
	require.Empty(t, last.OlderCursor)

	query.Text = "commit TIMED"
	result, err = service.Search(ctx, scope, query)
	require.NoError(t, err)
	require.Len(t, result.Hits, 1)
	require.Equal(t, trace, result.Hits[0].Message)
	require.Equal(t, instance, result.Hits[0].Instance)
	require.Equal(t, first, result.Hits[0].ArchiveID)
	hit := result.Hits[0]
	ref := archive.HitRef{ArchiveID: first, Entry: hit.Entry, Line: hit.Line}
	window, err := service.Context(ctx, scope, ref, 20)
	require.NoError(t, err)
	require.Len(t, window.Rows, 1, "adjacent records from other instances must not appear")
	require.Equal(t, trace, window.Rows[window.Anchor].Message)
	continuation := ref
	continuation.Line++
	_, err = service.Context(ctx, scope, continuation, 20)
	require.ErrorIs(t, err, archive.ErrHitNotFound)
	filter := ""
	filtered := archive.SearchQuery{From: query.From, To: query.To, Instance: &filter, Text: "late", IncludeInstances: true}
	matches, err := service.Search(ctx, scope, filtered)
	require.NoError(t, err)
	require.Equal(t, 1, matches.TotalMatches, "filter before taking the 200 newest")
	require.Equal(t, "late arrival", matches.Hits[0].Message)
	require.Equal(t, []string{"", instance, "worker"}, matches.Instances, "the picker covers the whole range regardless of selected instance, text or page")

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
	_, err = service.Search(ctx, scope, next)
	require.ErrorIs(t, err, archive.ErrArchiveUnavailable, "continuations must verify the ZIPs again")
	result, err = service.Search(ctx, scope, query)
	require.ErrorIs(t, err, archive.ErrArchiveUnavailable)
	require.Equal(t, archive.SearchResult{}, result)
	window, err = service.Context(ctx, scope, ref, 5)
	require.ErrorIs(t, err, archive.ErrArchiveUnavailable)
	require.Empty(t, window.Rows)
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

func TestArchiveSearchCursorAcrossTiedRecords(t *testing.T) {
	ctx := t.Context()
	pool, service, scope, dir := newArchiveFixture(t, ctx, "ArchivePagination")
	stamp := time.Date(2020, 3, 1, 12, 0, 0, 123456000, time.UTC)
	base := archive.SearchQuery{From: stamp.Add(-time.Second), To: stamp.Add(time.Second), Text: "record"}
	var ids []uuid.UUID
	for range 2 {
		var rows []logstore.Row
		for _, instance := range []string{"web", "worker"} {
			for seq := range 150 {
				rows = append(rows, logstore.Row{
					Scope: logstore.Scope{ClientID: scope.ClientID}, OccurredAt: stamp, Instance: instance,
					Sequence: int64(seq), Message: fmt.Sprintf("record %d\ntrace", seq),
				})
			}
		}
		_, err := testutils.InsertLogFixtures(ctx, pool, rows)
		require.NoError(t, err)
		job, err := service.Create(ctx, scope)
		require.NoError(t, err)
		ids = append(ids, job.Archive.ID)
	}

	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	var want []archive.SearchHit
	for _, id := range ids {
		for _, instance := range []string{"web", "worker"} {
			for i := range 150 {
				want = append(want, archive.SearchHit{
					ArchiveID: id, Entry: instance + "/logs_000000.log", Line: 2*i + 1, OccurredAt: stamp,
					Instance: instance, Message: fmt.Sprintf("record %d\ntrace", 149-i),
				})
			}
		}
	}
	query := base
	var firstCursor string
	for offset := 0; offset < len(want); offset += 200 {
		page, err := service.Search(ctx, scope, query)
		require.NoError(t, err)
		require.Equal(t, len(want), page.TotalMatches)
		require.Equal(t, offset, page.Skipped)
		require.Equal(t, 2, page.ArchivesScanned)
		for i := range page.Hits {
			page.Hits[i].OccurredAt = page.Hits[i].OccurredAt.UTC()
		}
		require.Equal(t, want[offset:offset+200], page.Hits)
		if offset+200 == len(want) {
			require.Empty(t, page.OlderCursor, "the last full page is final")
		} else {
			require.NotEmpty(t, page.OlderCursor)
		}
		query.OlderCursor = page.OlderCursor
		if offset == 0 {
			firstCursor = page.OlderCursor
			// A different replica can continue with just the shared secret and ZIPs.
			service = archive.NewArchiveService(pool, archive.NewDiskStore(dir), "archive-fixture-secret")
		}
	}
	query = base
	query.OlderCursor = firstCursor
	assertInvalid := func(q archive.SearchQuery) {
		t.Helper()
		r, err := service.Search(ctx, scope, q)
		require.ErrorIs(t, err, archive.ErrSearchCursor)
		require.Equal(t, archive.SearchResult{}, r)
	}
	at := query
	at.From = at.From.Add(time.Nanosecond)
	assertInvalid(at)
	text := query
	text.Text = "trace"
	assertInvalid(text)
	tampered := query
	tampered.OlderCursor = "!" + firstCursor
	assertInvalid(tampered)
	tampered.OlderCursor = "A" + firstCursor[1:]
	if tampered.OlderCursor == firstCursor {
		tampered.OlderCursor = "B" + firstCursor[1:]
	}
	assertInvalid(tampered)

	// An overlapping new archive invalidates the cursor even without new matches.
	_, err := testutils.InsertLogFixtures(ctx, pool, []logstore.Row{{Scope: logstore.Scope{ClientID: scope.ClientID}, OccurredAt: stamp, Message: "late"}})
	require.NoError(t, err)
	job, err := service.Create(ctx, scope)
	require.NoError(t, err)
	assertInvalid(query)
	fresh, err := service.Search(ctx, scope, base)
	require.NoError(t, err)
	require.Equal(t, len(want), fresh.TotalMatches)
	// The ID set is unchanged but the recorded ZIP digest differs.
	_, err = pool.Exec(ctx, `UPDATE archives SET sha256 = $1 WHERE id = $2`, make([]byte, 32), job.Archive.ID)
	require.NoError(t, err)
	query.OlderCursor = fresh.OlderCursor
	assertInvalid(query)
}
