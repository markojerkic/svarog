package types

type GetUserPageInput struct {
	Username string `json:"username"`
	Page     int64  `json:"page" default:"0"`
	Size     int64  `json:"size" default:"10"`
}

type RegisterForm struct {
	Username  string `json:"username" form:"username" validate:"required,gte=3"`
	Password  string `json:"password" form:"password" validate:"required,gte=8"`
	FirstName string `json:"first_name" form:"first_name" validate:"required,gte=3"`
	LastName  string `json:"last_name" form:"last_name" validate:"required,gte=3"`
}
