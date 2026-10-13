package auth

import "github.com/google/uuid"

type Role string

const (
	ADMIN Role = "admin"
	USER  Role = "user"
)

type User struct {
	ID                 uuid.UUID `json:"id"`
	FirstName          string    `json:"firstName"`
	LastName           string    `json:"lastName"`
	Username           string    `json:"username"`
	Password           string    `json:"-"`
	Role               Role      `json:"role"`
	NeedsPasswordReset bool      `json:"needsPasswordReset"`
	ProjectIDs         []string  `json:"projectIds,omitempty"`
}

type LoggedInUser struct {
	ID                 string   `json:"id"`
	Username           string   `json:"username"`
	Role               Role     `json:"role"`
	NeedsPasswordReset bool     `json:"needsPasswordReset"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
}
