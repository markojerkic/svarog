package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ClientId string `json:"clientId"`
	Project  string `json:"project"`
}

type StoredLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	LogLine        string             `bson:"log_line"`
	Timestamp      time.Time          `bson:"timestamp"`
	Client         StoredClient       `bson:"client"`
	SequenceNumber int                `bson:"sequence_number"`
}

type StoredClient struct {
	ProjectId string `bson:"project_id" json:"projectId"`
	ClientId  string `bson:"client_id" json:"clientId"`
	IpAddress string `bson:"ip_address" json:"ipAddress"`
}
