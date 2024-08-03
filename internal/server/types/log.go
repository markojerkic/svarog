package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StoredClient struct {
	ClientId  string `bson:"client_id" json:"clientId"`
	IpAddress string `bson:"ip_address" json:"ipAddress"`
}

type StoredLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	LogLine        string             `bson:"log_line"`
	Timestamp      time.Time          `bson:"timestamp"`
	Client         StoredClient       `bson:"client"`
	SequenceNumber int64              `bson:"sequence_number"`
}
