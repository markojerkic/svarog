package types

type GetUserPageInput struct {
	Username string `json:"username"`
	Page     int64  `json:"page" default:"0"`
	Size     int64  `json:"size" default:"10"`
}

type LoginForm struct {
	Username string `json:"username" form:"username" validate:"required,gte=3"`
	Password string `json:"password" form:"password" validate:"required,gte=8"`
}

type LoginFormWithToken struct {
	Token string `json:"token" form:"token" validate:"required,gte=5"`
}

type RegisterForm struct {
	Username  string `json:"username" form:"username" validate:"required,gte=3"`
	FirstName string `json:"firstName" form:"firstName" validate:"required,gte=3"`
	LastName  string `json:"lastName" form:"lastName" validate:"required,gte=3"`
}

type ResetPasswordForm struct {
	Password         string `json:"password" form:"password" validate:"required,gte=8"`
	RepeatedPassword string `json:"repeatedPassword" form:"repeatedPassword" validate:"required,gte=8"`
}
