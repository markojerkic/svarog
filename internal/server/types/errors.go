package types

type ApiError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}
