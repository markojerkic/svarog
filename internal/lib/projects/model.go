package projects

import "go.mongodb.org/mongo-driver/bson/primitive"

type Project struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name             string             `bson:"name" json:"name"`
	Clients          []string           `bson:"clients" json:"clients"`
	TotalStorageSize float64            `bson:"totalSizeMB" json:"totalStorageSize"`
}
