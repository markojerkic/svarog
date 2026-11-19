package auth

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/validation"
	"github.com/markojerkic/svarog/internal/server/types"
)

func ValidateUserForm(form types.CreateUserForm) error {
	fields := make(map[string]validation.Violation)
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"username", form.Username, types.UsernameMaxLength},
		{"firstName", form.FirstName, types.UserProfileNameMaxLength},
		{"lastName", form.LastName, types.UserProfileNameMaxLength},
	} {
		if violation := userTextError(field.value, 3, field.maximum); violation.Key != "" {
			fields[field.name] = violation
		}
	}
	if form.ID != "" && !canonicalUserUUID(form.ID) {
		fields["id"] = validation.Violation{Key: "users.invalid_id"}
	}
	if form.Role == "" {
		fields["role"] = validation.Violation{Key: "validation.required"}
	} else if form.Role != string(USER) && form.Role != string(ADMIN) {
		fields["role"] = validation.Violation{Key: "validation.one_of", Options: []string{"users.role_user", "users.role_admin"}}
	}
	for _, id := range form.ProjectIDs {
		if !canonicalUserUUID(id) {
			fields["projects"] = validation.Violation{Key: "projects.select_valid"}
			break
		}
	}
	if len(fields) > 0 {
		return &validation.Error{Key: "validation.failed", Fields: fields}
	}
	return nil
}

func ValidateUserPage(query types.GetUserPageInput) error {
	fields := make(map[string]validation.Violation)
	if query.Size < 1 || query.Size > 100 {
		fields["size"] = validation.Violation{Key: "validation.invalid"}
	}
	// ListUsers uses int32 LIMIT/OFFSET parameters. Validate before narrowing
	// the integers or multiplying, rather than exposing a persistence error.
	if query.Page < 0 || (query.Size > 0 && query.Page > math.MaxInt32/query.Size) {
		fields["page"] = validation.Violation{Key: "validation.invalid"}
	}
	if violation := userTextError(query.Search, 0, types.UserSearchMaxLength); violation.Key != "" {
		fields["search"] = violation
	}
	if len(fields) > 0 {
		return &validation.Error{Key: "request.invalid_parameters", Fields: fields}
	}
	return nil
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
		return errors.New("Password must contain between 8 and 72 bytes")
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

func canonicalUserUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
