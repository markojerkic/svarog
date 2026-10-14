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
