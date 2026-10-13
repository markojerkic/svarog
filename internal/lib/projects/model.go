package projects

import (
	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/types"
)

type Project struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Clients          []Client  `json:"clients"`
	TotalStorageSize float64   `json:"totalStorageSize"`
}

type Client struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (p Project) ClientNames() []string {
	names := make([]string, len(p.Clients))
	for i, client := range p.Clients {
		names[i] = client.Name
	}
	return names
}

func (p *Project) ToCreateProjectForm() types.CreateProjectForm {
	return types.CreateProjectForm{
		ID:      p.ID.String(),
		Name:    p.Name,
		Clients: p.ClientNames(),
	}
}
