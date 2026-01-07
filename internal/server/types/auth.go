package types

import (
	"github.com/markojerkic/svarog/internal/server/ui/components/combobox"
)

type GetUserPageInput struct {
	Page   int64  `json:"page" query:"page" default:"0"`
	Size   int64  `json:"size" query:"size" default:"10"`
	Search string `json:"search" query:"search"`
}

type LoginForm struct {
	Username string `json:"username" form:"username" validate:"required,gte=3"`
	Password string `json:"password" form:"password" validate:"required,gte=8"`
}

type LoginFormWithToken struct {
	Token string `json:"token" form:"token" query:"token" validate:"required,gte=5"`
}

type CreateUserForm struct {
	ID         string          `json:"id" form:"id"`
	Username   string          `json:"username" form:"username" validate:"required,gte=3"`
	FirstName  string          `json:"firstName" form:"firstName" validate:"required,gte=3"`
	LastName   string          `json:"lastName" form:"lastName" validate:"required,gte=3"`
	Role       string          `json:"role" form:"role" validate:"required,oneof=user admin"`
	ProjectIDs []string        `json:"projectIds" form:"projectIds"`
	Projects   []combobox.Item `json:"-" form:"-"`
}

type ResetPasswordForm struct {
	Password         string `json:"password" form:"password" validate:"required,gte=8"`
	RepeatedPassword string `json:"repeatedPassword" form:"repeatedPassword" validate:"required,gte=8"`
}
