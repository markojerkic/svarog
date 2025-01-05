package types

type GetUserPageInput struct {
	Username string `json:"username"`
	Page     int64  `json:"page" default:"0"`
	Size     int64  `json:"size" default:"10"`
}
