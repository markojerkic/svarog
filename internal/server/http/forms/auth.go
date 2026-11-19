package forms

type LoginForm struct {
	Username string `json:"username" form:"username" validate:"required,gte=3"`
	Password string `json:"password" form:"password" validate:"required,gte=8"`
	Redirect string `json:"redirect" form:"redirect"`
}

type LoginFormWithToken struct {
	Token string `json:"token" form:"token" query:"token" validate:"required,gte=5"`
}

type CreateUserForm struct {
	ID         string                `json:"id" form:"id"`
	Username   string                `json:"username" form:"username" validate:"required,gte=3"`
	FirstName  string                `json:"firstName" form:"firstName" validate:"required,gte=3"`
	LastName   string                `json:"lastName" form:"lastName" validate:"required,gte=3"`
	Role       string                `json:"role" form:"role" validate:"required,oneof=user admin"`
	ProjectIDs CommaSeparatedStrings `json:"projects" form:"projects"`
}

type ResetPasswordForm struct {
	Password         string `json:"password" form:"password" validate:"required,gte=8"`
	RepeatedPassword string `json:"repeatedPassword" form:"repeatedPassword" validate:"required,gte=8"`
}
