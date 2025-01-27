package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CreateProjectForm struct {
	Name    string   `json:"name" form:"name" validate:"required,gte=3"`
	Clients []string `json:"clients" form:"clients"`
}

type RemoveClientForm struct {
	ClientId  string `json:"clientId" form:"clientId" validate:"required"`
	ProjectId string `json:"projectId" form:"projectId" validate:"required"`
}

type AddClientForm struct {
	ClientName string `json:"clientName" form:"clientName" validate:"required"`
	ProjectId  string `json:"projectId" form:"projectId" validate:"required"`
}

type ProjectDto struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Clients     []string           `bson:"clients" json:"clients"`
	StorageSize int64              `json:"storageSize"`
}
