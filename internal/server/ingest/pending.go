package ingest

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/nats-io/nats.go/jetstream"
)

type pendingLog struct {
	message   jetstream.Msg
	input     logstore.Event
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

func decode(msg jetstream.Msg) (logstore.Event, rejectReason) {
	var empty logstore.Event
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
	return logstore.Event{
		Key:        logevent.MessageKey(msg.Subject(), msg.Data()),
		Scope:      logstore.Scope{ProjectID: projectID, ClientID: clientID},
		Instance:   line.InstanceId,
		OccurredAt: line.Timestamp,
		Sequence:   int64(line.Sequence),
		Message:    line.Message,
	}, ""
}
