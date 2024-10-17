package auth

import "go.mongodb.org/mongo-driver/bson/primitive"

type Role string

const (
	ADMIN Role = "admin"
	USER  Role = "user"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Username string             `bson:"username"`
	Password string             `bson:"password" json:"-"`
	Role     Role               `bson:"role"`
}

type LoggedInUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}
