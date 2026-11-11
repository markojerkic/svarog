package db

import "github.com/markojerkic/svarog/internal/server/types"

type IngestLog struct {
	EventKey [32]byte
	Log      types.StoredLog
}
