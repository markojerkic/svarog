package types

type CreateProjectForm struct {
	Name    string   `json:"name" form:"name" validate:"required,gte=3"`
	Clients []string `json:"clients" form:"clients"`
}

type RemoveClientForm struct {
	ClientId  string `json:"clientId" form:"clientId" validate:"required"`
	ProjectId string `json:"projectId" form:"projectId" validate:"required"`
}

type AddClientForm struct {
	ClientId  string `json:"clientId" form:"clientId" validate:"required"`
	ProjectId string `json:"projectId" form:"projectId" validate:"required"`
}
