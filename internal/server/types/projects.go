package types

import (
	"github.com/google/uuid"
)

const (
	ProjectNameMaxLength   = 255
	ClientNameMaxLength    = 255
	ProjectSearchMaxLength = 1024
)

type GetProjectPageInput struct {
	Search string `json:"search" query:"search"`
	Page   int64  `json:"page" query:"page" default:"0" validate:"gte=0"`
	Size   int64  `json:"size" query:"size" default:"10" validate:"gte=1,lte=100"`
}

type CreateProjectForm struct {
	ID      string   `json:"id" form:"id"`
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
	ID          uuid.UUID `json:"id"`
	Name        string    `bson:"name" json:"name"`
	Clients     []string  `bson:"clients" json:"clients"`
	StorageSize int64     `json:"storageSize"`
}
