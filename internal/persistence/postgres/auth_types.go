package postgres

import (
	"time"

	"github.com/google/uuid"
)

type LoginTokenInput struct {
	Hash      []byte
	ExpiresAt time.Time
}

type CreateUserWithTokenInput struct {
	ID                 uuid.UUID
	Username           string
	FirstName          string
	LastName           string
	PasswordHash       string
	Role               string
	NeedsPasswordReset bool
	Token              LoginTokenInput
	ProjectIDs         []uuid.UUID
}

type UpdateUserProfileWithProjectsInput struct {
	ID         uuid.UUID
	Username   string
	FirstName  string
	LastName   string
	Role       string
	ProjectIDs []uuid.UUID
}

type ReplaceLoginTokenInput struct {
	UserID       uuid.UUID
	PasswordHash string
	Token        LoginTokenInput
}
