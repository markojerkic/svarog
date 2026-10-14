package types

import "time"

type ArchiveSetting struct {
	ClientID         string
	ProjectID        string
	ArhiveAfterWeeks int
	UpdatedAt        time.Time
}
