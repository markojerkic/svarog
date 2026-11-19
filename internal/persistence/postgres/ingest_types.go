package postgres

import "github.com/google/uuid"

type IngestLogInsert struct {
	ProjectID uuid.UUID
	EventKey  [32]byte
	Log       LogInsert
}

type IngestLogStatus uint8

const (
	IngestLogInserted IngestLogStatus = iota + 1
	IngestLogDuplicate
	IngestLogRejectedClient
)

type IngestLogResult struct {
	Ordinal int64 // One-based position in the input batch.
	ID      int64 // Set only for a newly inserted row.
	Status  IngestLogStatus
}
