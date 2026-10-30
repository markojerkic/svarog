package postgres

import (
	"time"

	"github.com/google/uuid"
)

type LogInsert struct {
	ClientID       uuid.UUID
	InstanceName   string
	OccurredAt     time.Time
	SequenceNumber int64
	Message        string
}

type LogRow struct {
	ID             int64
	ClientID       uuid.UUID
	ProjectID      uuid.UUID
	InstanceName   string
	OccurredAt     time.Time
	SequenceNumber int64
	Message        string
}

type LogReadQuery struct {
	ProjectID       uuid.UUID
	ClientID        uuid.UUID
	Instances       []string
	FilterInstances bool
	PageSize        int32
	HasCursor       bool
	Backward        bool
	Inclusive       bool
	CursorAt        time.Time
	CursorSequence  int64
	CursorID        *int64
}

type LogSearchQuery struct {
	LogReadQuery
	PrimaryPattern    string
	RemainingPatterns []string
}
