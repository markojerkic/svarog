package archive

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

const DefaultContextLines = 20
const MaxContextLines = 100

var ErrHitNotFound = errors.New("archived log record not found")

type HitRef struct {
	ArchiveID uuid.UUID
	Entry     string
	Line      int // First physical line of the record in Entry (1-based).
}

type HitContext struct {
	Archive   Archive
	Rows      []SearchHit // oldest first
	Anchor    int         // index into Rows
	OlderEdge bool        // window reaches the first record of this instance
	NewerEdge bool        // window reaches the last record of this instance
}

func (a *Service) Context(ctx context.Context, scope Scope, ref HitRef, lines int) (HitContext, error) {
	if lines < 1 || lines > MaxContextLines || ref.Line < 1 || len(ref.Entry) > 4096 {
		return HitContext{}, ErrInvalidInput
	}
	instance, err := entryInstance(ref.Entry)
	if err != nil {
		return HitContext{}, ErrHitNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	d, err := a.Open(ctx, scope, ref.ArchiveID)
	if err != nil {
		return HitContext{}, err
	}
	// The writer streams each instance newest first across consecutive ZIP members.
	// Keep a sliding window before the anchor, then collect up to lines older rows.
	rows := make([]SearchHit, 0, 2*lines+1)
	anchor, size := -1, 0
	var newerSeen int
	var moreOlder bool
	remaining := uint64(maxExpanded)
	err = scanDownload(ctx, d, &remaining, func(hit SearchHit) error {
		if hit.Instance != instance {
			return nil
		}
		switch {
		case hit.Entry == ref.Entry && hit.Line == ref.Line:
			if anchor >= 0 {
				return ErrArchiveUnavailable
			}
			if len(rows) != min(lines, newerSeen) {
				return ErrSearchLimit
			}
			anchor = len(rows)
		case anchor < 0:
			newerSeen++
			for len(rows) > 0 && (len(rows) == lines || size+len(hit.Message) > maxResultText) {
				size -= len(rows[0].Message)
				rows = slices.Delete(rows, 0, 1)
			}
		case anchor >= 0 && len(rows)-anchor-1 == lines:
			moreOlder = true
			return nil
		}
		size += len(hit.Message)
		if size > maxResultText {
			return ErrSearchLimit
		}
		rows = append(rows, hit)
		return nil
	})
	if err != nil {
		return HitContext{}, err
	}
	if anchor < 0 {
		return HitContext{}, ErrHitNotFound
	}
	slices.Reverse(rows)
	return HitContext{Archive: d.Archive, Rows: rows, Anchor: len(rows) - 1 - anchor, OlderEdge: !moreOlder, NewerEdge: newerSeen <= lines}, nil
}
