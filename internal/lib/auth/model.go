package auth

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	ADMIN Role = "admin"
	USER  Role = "user"
)

type User struct {
	ID                 primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	FirstName          string               `bson:"firstName" json:"firstName"`
	LastName           string               `bson:"lastName" json:"lastName"`
	Username           string               `bson:"username" json:"username"`
	Password           string               `bson:"password" json:"-"`
	Role               Role                 `bson:"role" json:"role"`
	NeedsPasswordReset bool                 `bson:"needs_password_reset" json:"needsPasswordReset"`
	LoginTokens        []string             `bson:"login_tokens" json:"-"`
	ProjectIDs         []primitive.ObjectID `bson:"project_ids,omitempty" json:"projectIds,omitempty"`
}

type LoggedInUser struct {
	ID                 string   `json:"id"`
	Username           string   `json:"username"`
	Role               Role     `json:"role"`
	NeedsPasswordReset bool     `json:"needsPasswordReset"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
}

type Session struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	UserID   primitive.ObjectID `bson:"user_id"`
	Modified time.Time          `bson:"modified"`
}
