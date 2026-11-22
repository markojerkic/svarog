package archive

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

const (
	maxSearchArchives = 128
	maxCompressed     = 256 << 20
	maxExpanded       = 512 << 20
	maxHits           = 200
	maxResultText     = 8 << 20
	MaxSearchText     = 256
)

type SearchQuery struct {
	From, To    time.Time
	Text        string
	OlderCursor string
}

type SearchHit struct {
	ArchiveID  uuid.UUID
	Entry      string
	Line       int
	OccurredAt time.Time
	Instance   string
	Message    string
}

type SearchResult struct {
	Hits            []SearchHit
	TotalMatches    int
	ArchivesScanned int
	Skipped         int    // matches on previous pages
	OlderCursor     string // empty when all matches have been returned
}

func (a *Service) Search(ctx context.Context, scope Scope, query SearchQuery) (SearchResult, error) {
	if err := validateScope(scope); err != nil {
		return SearchResult{}, err
	}
	if query.From.IsZero() || query.To.IsZero() || !query.To.After(query.From) || utf8.RuneCountInString(query.Text) > MaxSearchText {
		return SearchResult{}, ErrInvalidInput
	}
	search, err := logsearch.Parse(query.Text)
	if err != nil {
		return SearchResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if _, err := getPolicy(ctx, a.pool, scope); err != nil {
		return SearchResult{}, err
	}
	candidates, err := sqlc.New(a.pool).ListSearchArchives(ctx, sqlc.ListSearchArchivesParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID, RangeStart: query.From, RangeEnd: query.To, CandidateLimit: maxSearchArchives + 1,
	})
	if err != nil {
		return SearchResult{}, err
	}
	cursorKey := searchCursorKey(a.cursorSecret, scope, query, candidates)
	state := searchScan{query: query, terms: search.Terms, remaining: maxExpanded}
	if query.OlderCursor != "" {
		state.after, err = decodeSearchCursor(cursorKey, query.OlderCursor)
		if err != nil {
			return SearchResult{}, err
		}
	}
	if len(candidates) > maxSearchArchives {
		return SearchResult{}, ErrSearchLimit
	}
	var compressed int64
	for _, row := range candidates {
		if row.SizeBytes.Int64 > maxCompressed-compressed {
			return SearchResult{}, ErrSearchLimit
		}
		compressed += row.SizeBytes.Int64
	}
	for _, row := range candidates {
		download, err := a.Open(ctx, scope, row.ID)
		if errors.Is(err, ErrArchiveNotFound) {
			return SearchResult{}, ErrSearchCursor
		}
		if err != nil {
			return SearchResult{}, err
		}
		// The metadata used for the cursor and byte budget must describe
		// the same ZIP we're about to verify, even across the two reads.
		if download.Archive.SizeBytes != row.SizeBytes.Int64 || !bytes.Equal(download.Archive.SHA256[:], row.Sha256) {
			download.Content.Close()
			return SearchResult{}, ErrSearchCursor
		}
		if err := state.scanDownload(ctx, download); err != nil {
			return SearchResult{}, fmt.Errorf("scan archive %s: %w", row.ID, err)
		}
		state.result.ArchivesScanned++
	}
	if err := ctx.Err(); err != nil {
		return SearchResult{}, err
	}
	if r := &state.result; r.Skipped+len(r.Hits) < r.TotalMatches {
		r.OlderCursor = encodeSearchCursor(cursorKey, r.Hits[len(r.Hits)-1])
	}
	return state.result, nil
}

type searchScan struct {
	query      SearchQuery
	after      *SearchHit
	terms      []string
	remaining  uint64 // expanded byte budget across all ZIPs
	resultSize int
	result     SearchResult
}

func (s *searchScan) scanDownload(ctx context.Context, d Download) error {
	defer d.Content.Close()
	file, err := os.CreateTemp("", "svarog-search-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(searchReader{ctx, d.Content}, d.Archive.SizeBytes+1))
	if err != nil {
		return err
	}
	if n != d.Archive.SizeBytes || !bytes.Equal(hash.Sum(nil), d.Archive.SHA256[:]) {
		return ErrArchiveUnavailable
	}
	zr, err := zip.NewReader(file, n)
	if err != nil {
		return err
	}
	var count int64
	for _, entry := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.UncompressedSize64 > s.remaining {
			return ErrSearchLimit
		}
		s.remaining -= entry.UncompressedSize64
		read, err := scanLogEntry(ctx, entry, d.Archive.ID, s.match)
		if err != nil {
			return err
		}
		count += read
	}
	if count != d.Archive.LogCount {
		return fmt.Errorf("decoded %d of %d records: %w", count, d.Archive.LogCount, ErrArchiveUnavailable)
	}
	return nil
}

type searchReader struct {
	ctx context.Context
	io.Reader
}

func (r searchReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func compareHits(a, b SearchHit) int {
	return cmp.Or(b.OccurredAt.Compare(a.OccurredAt), bytes.Compare(a.ArchiveID[:], b.ArchiveID[:]), strings.Compare(a.Entry, b.Entry), cmp.Compare(a.Line, b.Line))
}

func (s *searchScan) match(hit SearchHit) error {
	if hit.OccurredAt.Before(s.query.From) || !hit.OccurredAt.Before(s.query.To) {
		return nil
	}
	if len(s.terms) != 0 {
		text := strings.ToLower(hit.Message)
		for _, term := range s.terms {
			if !strings.Contains(text, term) {
				return nil
			}
		}
	}
	s.result.TotalMatches++
	if s.after != nil && compareHits(hit, *s.after) <= 0 {
		s.result.Skipped++
		return nil
	}
	hits := s.result.Hits
	at, _ := slices.BinarySearchFunc(hits, hit, compareHits)
	if at == maxHits {
		return nil
	}
	if len(hits) == maxHits {
		s.resultSize -= len(hits[maxHits-1].Message)
		hits = hits[:maxHits-1]
	}
	s.resultSize += len(hit.Message)
	if s.resultSize > maxResultText {
		return ErrSearchLimit
	}
	s.result.Hits = slices.Insert(hits, at, hit)
	return nil
}
