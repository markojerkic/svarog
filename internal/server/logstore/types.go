package logstore

import (
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
)

type Scope struct {
	ProjectID uuid.UUID
	ClientID  uuid.UUID
}

type InstanceCursor struct {
	Day  string `json:"day"`
	Name string `json:"name"`
}

type InstanceOptions struct {
	Names []string        `json:"names"`
	Next  *InstanceCursor `json:"next"`
}

type Row struct {
	ID         int64
	Scope      Scope
	Instance   string
	OccurredAt time.Time
	Sequence   int64
	Message    string
}

type Event struct {
	Key        [32]byte
	Scope      Scope
	Instance   string
	OccurredAt time.Time
	Sequence   int64
	Message    string
}

type Outcome uint8

const (
	Inserted Outcome = iota + 1
	Duplicate
	RejectedClient
)

type Result struct {
	Outcome Outcome
	Row     Row // Set only for Inserted.
}

type Cursor struct {
	OccurredAt time.Time
	Sequence   int64
	ID         int64
}

// TimeRange includes From and excludes To.
type TimeRange struct {
	From time.Time
	To   time.Time
}

type PageQuery struct {
	Scope     Scope
	Instances []string // Nil selects all instances. A non-nil empty slice selects none.
	Range     *TimeRange
	// Use at most one cursor. No cursor requests the newest page.
	Older  *Cursor
	Newer  *Cursor
	Search *logsearch.Query
	Limit  int

	// AnchorID resolves the instance of a context continuation if the URL still
	// supplies its anchor. The existing selection applies if the anchor is gone.
	AnchorID int64
}

type Page struct {
	Rows           []Row // Newest first.
	Older          *Cursor
	Newer          *Cursor
	AnchorInstance *string // Set when an authorized context anchor was found.
}
