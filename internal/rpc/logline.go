package rpc

import "time"

type LogLine struct {
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Sequence   int       `json:"sequence"`
	InstanceId string    `json:"instanceId"`
}
