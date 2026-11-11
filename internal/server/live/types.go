// Package live distributes committed logs and renders them only on gateways
// with interested viewers. Core NATS delivery is best effort, not a replay log.
package live

import (
	"encoding/base64"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logevent"
)

const (
	maxBatchRows         = 100
	maxBatchBytes        = 128 << 10
	maxQueueRows         = 8192
	maxQueueBytes        = 8 << 20
	maxQueueJobs         = 64
	maxViewerMessages    = 100
	maxViewerBytes       = 1 << 20
	maxTransitionBatches = 8192
	maxInstanceBytes     = logevent.MaxInstanceIDBytes
)

var (
	ErrLiveResync      = errors.New("live log view requires a fresh snapshot")
	ErrInvalidSelector = errors.New("invalid live log instance selection")
	ErrClosed          = errors.New("live logs are closed")
)

type Scope struct {
	ProjectID uuid.UUID
	ClientID  uuid.UUID
}

// An empty selection watches all instances; []string{""} watches the unnamed
// instance. Open copies the selection, so callers retain ownership of it.
type Selector struct {
	Instances []string
}

// NewSelector bounds subscription fanout and URL/subject size without treating
// punctuation, whitespace or an empty instance name as NATS syntax.
func NewSelector(instances []string) (Selector, error) {
	if len(instances) > 32 {
		return Selector{}, ErrInvalidSelector
	}
	names := slices.Clone(instances)
	slices.Sort(names)
	names = slices.Compact(names)
	total := 0
	for _, name := range names {
		total += len(name)
		if len(name) > maxInstanceBytes || total > 4096 {
			return Selector{}, ErrInvalidSelector
		}
	}
	return Selector{Instances: names}, nil
}

func (s Selector) matches(instance string) bool {
	return len(s.Instances) == 0 || slices.Contains(s.Instances, instance)
}

func (s Scope) prefix() string {
	return "committed.logs." + s.ProjectID.String() + "." + s.ClientID.String() + "."
}

func instanceToken(instance string) string {
	return "i" + base64.RawURLEncoding.EncodeToString([]byte(instance))
}

type committedRow struct {
	ID         int64  `json:"id,string"`
	OccurredUs int64  `json:"occurredUs,string"`
	Sequence   int    `json:"sequence,string"`
	Message    string `json:"message"`
}

// ID identifies one publication during overlapping subscription transitions;
// row IDs identify database rows and are never interpreted as commit offsets.
type committedBatch struct {
	Version   int            `json:"version"`
	ID        string         `json:"batchId"`
	ProjectID string         `json:"projectId"`
	ClientID  string         `json:"clientId"`
	Instance  string         `json:"instance"`
	Resync    bool           `json:"resync,omitempty"`
	Rows      []committedRow `json:"rows"`
}
