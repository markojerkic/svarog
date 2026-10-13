package postgres

import "github.com/google/uuid"

type ProjectWithClientsInput struct {
	ID      uuid.UUID
	Name    string
	Clients []string
}
