package types

type ApiError struct {
	error
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}

func NewApiError(message string, fields ...map[string]string) ApiError {
	var fieldMap map[string]string
	if len(fields) > 0 {
		fieldMap = fields[0]
	} else {
		fieldMap = make(map[string]string)
	}

	return ApiError{
		Message: message,
		Fields:  fieldMap,
	}
}
