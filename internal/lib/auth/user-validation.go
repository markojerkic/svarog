package auth

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/validation"
)

const (
	UsernameMaxLength        = 255
	UserProfileNameMaxLength = 255
	UserSearchMaxLength      = 1024
)

func ValidateUserInput(input UserInput) error {
	return validateUserInput(input, false)
}

func validateUserInput(input UserInput, invalidID bool) error {
	fields := make(map[string]validation.Violation)
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"username", input.Username, UsernameMaxLength},
		{"firstName", input.FirstName, UserProfileNameMaxLength},
		{"lastName", input.LastName, UserProfileNameMaxLength},
	} {
		if violation := userTextError(field.value, 3, field.maximum); violation.Key != "" {
			fields[field.name] = violation
		}
	}
	if invalidID {
		fields["id"] = validation.Violation{Key: "users.invalid_id"}
	}
	if input.Role == "" {
		fields["role"] = validation.Violation{Key: "validation.required"}
	} else if input.Role != USER && input.Role != ADMIN {
		fields["role"] = validation.Violation{Key: "validation.one_of", Options: []string{"users.role_user", "users.role_admin"}}
	}
	for _, id := range input.ProjectIDs {
		if id == uuid.Nil {
			fields["projects"] = validation.Violation{Key: "projects.select_valid"}
			break
		}
	}
	if len(fields) > 0 {
		return &validation.Error{Key: "validation.failed", Fields: fields}
	}
	return nil
}

func NewPageQuery(search string, page, size int64) (PageQuery, error) {
	fields := make(map[string]validation.Violation)
	if size < 1 || size > 100 {
		fields["size"] = validation.Violation{Key: "validation.invalid"}
	}
	// ListUsers uses int32 LIMIT/OFFSET parameters. Validate before narrowing
	// the integers or multiplying, rather than exposing a persistence error.
	if page < 0 || (size > 0 && page > math.MaxInt32/size) {
		fields["page"] = validation.Violation{Key: "validation.invalid"}
	}
	if violation := userTextError(search, 0, UserSearchMaxLength); violation.Key != "" {
		fields["search"] = violation
	}
	if len(fields) > 0 {
		return PageQuery{}, &validation.Error{Key: "request.invalid_parameters", Fields: fields}
	}
	return PageQuery{search: search, page: page, size: size}, nil
}

// ValidateNewPassword rejects a password whose meaningful content is shorter than 8
// characters. Eight spaces pass a raw length check and can then be used to log in.
// The stored value is not trimmed: surrounding spaces in an otherwise long password stay significant.
func ValidateNewPassword(password string) error {
	if !utf8.ValidString(password) || strings.ContainsRune(password, '\x00') {
		return ErrPasswordBlank
	}
	if utf8.RuneCountInString(strings.TrimSpace(password)) < 8 {
		return ErrPasswordBlank
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}
	return nil
}

func userTextError(value string, minimum, maximum int) validation.Violation {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return validation.Violation{Key: "validation.invalid"}
	}
	trimmed := strings.TrimSpace(value)
	if minimum > 0 && trimmed == "" {
		return validation.Violation{Key: "validation.required"}
	}
	if utf8.RuneCountInString(trimmed) < minimum {
		return validation.Violation{Key: "validation.min_length", Count: minimum}
	}
	if utf8.RuneCountInString(value) > maximum {
		return validation.Violation{Key: "validation.max_length", Count: maximum}
	}
	return validation.Violation{}
}
