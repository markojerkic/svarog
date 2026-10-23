package auth

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
)

func ValidateUserForm(ctx context.Context, form types.CreateUserForm) error {
	fields := make(map[string]string)
	for _, field := range []struct {
		name    string
		value   string
		maximum int
	}{
		{"username", form.Username, types.UsernameMaxLength},
		{"firstName", form.FirstName, types.UserProfileNameMaxLength},
		{"lastName", form.LastName, types.UserProfileNameMaxLength},
	} {
		if message := userTextError(ctx, field.value, 3, field.maximum); message != "" {
			fields[field.name] = message
		}
	}
	if form.ID != "" && !canonicalUserUUID(form.ID) {
		fields["id"] = i18n.T(ctx, "users.invalid_id")
	}
	if form.Role == "" {
		fields["role"] = i18n.T(ctx, "validation.required")
	} else if form.Role != string(USER) && form.Role != string(ADMIN) {
		fields["role"] = i18n.Format(ctx, i18n.Message{
			ID: "validation.one_of", Data: map[string]any{"Options": i18n.T(ctx, "users.role_user") + ", " + i18n.T(ctx, "users.role_admin")},
		})
	}
	for _, id := range form.ProjectIDs {
		if !canonicalUserUUID(id) {
			fields["projects"] = i18n.T(ctx, "projects.select_valid")
			break
		}
	}
	if len(fields) > 0 {
		return types.NewApiError(i18n.T(ctx, "validation.failed"), fields)
	}
	return nil
}

func ValidateUserPage(ctx context.Context, query types.GetUserPageInput) error {
	fields := make(map[string]string)
	if query.Size < 1 || query.Size > 100 {
		fields["size"] = i18n.T(ctx, "validation.invalid")
	}
	// ListUsers uses int32 LIMIT/OFFSET parameters. Validate before narrowing
	// the integers or multiplying, rather than exposing a persistence error.
	if query.Page < 0 || (query.Size > 0 && query.Page > math.MaxInt32/query.Size) {
		fields["page"] = i18n.T(ctx, "validation.invalid")
	}
	if message := userTextError(ctx, query.Search, 0, types.UserSearchMaxLength); message != "" {
		fields["search"] = message
	}
	if len(fields) > 0 {
		return types.NewApiError(i18n.T(ctx, "request.invalid_parameters"), fields)
	}
	return nil
}

func userTextError(ctx context.Context, value string, minimum, maximum int) string {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return i18n.T(ctx, "validation.invalid")
	}
	trimmed := strings.TrimSpace(value)
	if minimum > 0 && trimmed == "" {
		return i18n.T(ctx, "validation.required")
	}
	if utf8.RuneCountInString(trimmed) < minimum {
		return i18n.Format(ctx, i18n.Message{ID: "validation.min_length", Data: map[string]any{"Count": minimum}, Count: &minimum})
	}
	if utf8.RuneCountInString(value) > maximum {
		return i18n.Format(ctx, i18n.Message{ID: "validation.max_length", Data: map[string]any{"Count": maximum}, Count: &maximum})
	}
	return ""
}

func canonicalUserUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
