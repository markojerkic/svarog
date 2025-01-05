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
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FirstName string             `bson:"first_name" json:"first_name"`
	LastName  string             `bson:"last_name" json:"last_name"`
	Username  string             `bson:"username" json:"username"`
	Password  string             `bson:"password" json:"-"`
	Role      Role               `bson:"role" json:"role"`
}

type LoggedInUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}

type Session struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	UserID   primitive.ObjectID `bson:"user_id"`
	Modified time.Time          `bson:"modified"`
}
