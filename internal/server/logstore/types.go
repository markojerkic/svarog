package logstore

import (
	"time"

	"github.com/google/uuid"
)

type Scope struct {
	ProjectID uuid.UUID
	ClientID  uuid.UUID
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
