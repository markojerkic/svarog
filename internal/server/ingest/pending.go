package ingest

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
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

func (s *Service) active(ctx context.Context, projectID, clientID uuid.UUID) (bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.queries.ActiveProjectClientExists(queryCtx, sqlc.ActiveProjectClientExistsParams{ProjectID: projectID, ID: clientID})
}

func (s *Service) decode(ctx context.Context, msg jetstream.Msg) (db.IngestLog, rejectReason, error) {
	var empty db.IngestLog
	parts := strings.Split(msg.Subject(), ".")
	if len(parts) != 3 || parts[0] != "logs" {
		return empty, rejectSubject, nil
	}
	projectID, projectErr := uuid.Parse(parts[1])
	clientID, clientErr := uuid.Parse(parts[2])
	if projectErr != nil || clientErr != nil || projectID == uuid.Nil || clientID == uuid.Nil || projectID.String() != parts[1] || clientID.String() != parts[2] {
		return empty, rejectSubject, nil
	}
	var line rpc.LogLine
	if err := json.Unmarshal(msg.Data(), &line); err != nil {
		return empty, rejectPayload, nil
	}
	if err := logevent.Validate(line); err != nil {
		return empty, rejectFields, nil
	}
	active, err := s.active(ctx, projectID, clientID)
	if err != nil {
		return empty, "", err
	}
	if !active {
		return empty, rejectInactive, nil
	}
	return db.IngestLog{
		EventKey: logevent.MessageKey(msg.Subject(), msg.Data()),
		Log: types.StoredLog{
			LogLine: line.Message, Timestamp: line.Timestamp, SequenceNumber: line.Sequence,
			Client: types.StoredClient{ProjectId: projectID.String(), ClientId: clientID.String(), InstanceId: line.InstanceId},
		},
	}, "", nil
}
