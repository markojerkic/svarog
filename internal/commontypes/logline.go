package commontypes

import "time"

type LogLineDto struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Sequence  int       `json:"sequence"`
}
