package archive

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// Archiver manages client policies and completed archives. It validates client
// ownership, but callers are responsible for user authorization.
type Archiver interface {
	// GetPolicy returns a disabled policy when the client has no rule.
	GetPolicy(ctx context.Context, scope Scope) (Policy, error)
	// SetPolicy replaces the rule idempotently. Disabling preserves archives and
	// does not cancel an in-progress Create; changes apply to the next build.
	SetPolicy(ctx context.Context, scope Scope, policy Policy) error
	// Create synchronously archives the fixed set of eligible logs. Only logs
	// safely written to the completed archive are deleted. An error does not
	// imply rollback: an interrupted commit can have an uncertain outcome.
	// Missing rules, empty cohorts and concurrent builds return skip outcomes,
	// not errors. Recovery runs under exclusive client ownership.
	Create(ctx context.Context, scope Scope) (CreateResult, error)
	// List returns completed archives ordered by CreatedAt DESC, ID DESC.
	List(ctx context.Context, scope Scope, query ListQuery) (Page, error)
	// Open returns completed archive content. The caller must close it and
	// handle read errors; the stream does not promise seeking or Range support.
	Open(ctx context.Context, scope Scope, archiveID uuid.UUID) (Download, error)
}

type Scope struct {
	ProjectID uuid.UUID
	ClientID  uuid.UUID
}

// MaxArchiveAfterWeeks bounds the cutoff calculation to a time.Duration.
const MaxArchiveAfterWeeks = int((1<<63 - 1) / (7 * 24 * time.Hour))

type Policy struct {
	Enabled           bool
	ArchiveAfterWeeks int // ignored when disabled; otherwise 1..MaxArchiveAfterWeeks
}

// Archive describes a completed ZIP without exposing its storage location.
type Archive struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	FileName  string
	SizeBytes int64
	SHA256    [32]byte
	LogCount  int64
	CutoffAt  time.Time
	FromAt    time.Time
	ToAt      time.Time
	CreatedAt time.Time
}

type CreateOutcome string

const (
	Created         CreateOutcome = "created"
	SkippedNoLogs   CreateOutcome = "skipped_no_logs"
	SkippedDisabled CreateOutcome = "skipped_disabled"
	SkippedBusy     CreateOutcome = "skipped_busy"
)

type CreateResult struct {
	Outcome CreateOutcome
	Archive *Archive // non-nil only for Created
}

type ListQuery struct {
	Limit  int     // zero defaults to 20; otherwise 1..100
	Before *Cursor // exclusive; nil starts at the newest archive
}

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type Page struct {
	Items []Archive
	Next  *Cursor // nil when there are no more archives
}

type Download struct {
	Archive Archive
	Content io.ReadCloser
}
