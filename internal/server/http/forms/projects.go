package forms

type PageInput struct {
	Search string `json:"search" query:"search"`
	Page   int64  `json:"page" query:"page" default:"0" validate:"gte=0"`
	Size   int64  `json:"size" query:"size" default:"10" validate:"gte=1,lte=100"`
}

type CreateProjectForm struct {
	ID      string   `json:"id" form:"id"`
	Name    string   `json:"name" form:"name" validate:"required,gte=3"`
	Clients []string `json:"clients" form:"clients"`
}

type CredentialGenerationForm struct {
	ProjectID string       `form:"projectId" validate:"required"`
	ClientID  string       `form:"clientId" validate:"required"`
	Expiry    NullableDate `form:"expiry"`
}
