package types

type ApiError struct {
	error
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func NewApiError(message string) ApiError {
	return ApiError{
		Message: message,
		Fields:  make(map[string]string),
	}
}
