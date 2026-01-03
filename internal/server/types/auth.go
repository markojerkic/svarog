package types

type GetUserPageInput struct {
	Username string `json:"username" query:"username"`
	Page     int64  `json:"page" query:"page" default:"0"`
	Size     int64  `json:"size" query:"size" default:"10"`
}

type LoginForm struct {
	Username string `json:"username" form:"username" validate:"required,gte=3"`
	Password string `json:"password" form:"password" validate:"required,gte=8"`
}

type LoginFormWithToken struct {
	Token string `json:"token" form:"token" query:"token" validate:"required,gte=5"`
}

type RegisterForm struct {
	Username  string `json:"username" form:"username" validate:"required,gte=3"`
	FirstName string `json:"firstName" form:"firstName" validate:"required,gte=3"`
	LastName  string `json:"lastName" form:"lastName" validate:"required,gte=3"`
}

type CreateUserForm struct {
	ID        string `json:"id" form:"id"`
	Username  string `json:"username" form:"username" validate:"required,gte=3"`
	FirstName string `json:"firstName" form:"firstName" validate:"required,gte=3"`
	LastName  string `json:"lastName" form:"lastName" validate:"required,gte=3"`
	Role      string `json:"role" form:"role" validate:"required,oneof=user admin"`
}

type ResetPasswordForm struct {
	Password         string `json:"password" form:"password" validate:"required,gte=8"`
	RepeatedPassword string `json:"repeatedPassword" form:"repeatedPassword" validate:"required,gte=8"`
}
