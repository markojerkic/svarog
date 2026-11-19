package ingest

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/nats-io/nats.go/jetstream"
)

type pendingLog struct {
	message   jetstream.Msg
	input     db.IngestLog
	wireBytes int
}

type rejectReason string

const (
	rejectSubject  rejectReason = "invalid_subject"
	rejectPayload  rejectReason = "invalid_payload"
	rejectFields   rejectReason = "invalid_fields"
	rejectInactive rejectReason = "inactive_client"
	rejectData     rejectReason = "permanent_data_error"
)

func decode(msg jetstream.Msg) (db.IngestLog, rejectReason) {
	var empty db.IngestLog
	parts := strings.Split(msg.Subject(), ".")
	if len(parts) != 3 || parts[0] != "logs" {
		return empty, rejectSubject
	}
	projectID, projectErr := uuid.Parse(parts[1])
	clientID, clientErr := uuid.Parse(parts[2])
	if projectErr != nil || clientErr != nil || projectID == uuid.Nil || clientID == uuid.Nil || projectID.String() != parts[1] || clientID.String() != parts[2] {
		return empty, rejectSubject
	}
	var line rpc.LogLine
	if err := json.Unmarshal(msg.Data(), &line); err != nil {
		return empty, rejectPayload
	}
	if err := logevent.Validate(line); err != nil {
		return empty, rejectFields
	}
	return db.IngestLog{
		EventKey: logevent.MessageKey(msg.Subject(), msg.Data()),
		Log: types.StoredLog{
			LogLine: line.Message, Timestamp: line.Timestamp, SequenceNumber: line.Sequence,
			Client: types.StoredClient{ProjectId: parts[1], ClientId: parts[2], InstanceId: line.InstanceId},
		},
	}, ""
}
