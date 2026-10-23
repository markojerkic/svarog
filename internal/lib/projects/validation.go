package projects

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
)

func ValidateProjectForm(ctx context.Context, form types.CreateProjectForm) error {
	if form.ID != "" {
		id, err := uuid.Parse(form.ID)
		if err != nil || id == uuid.Nil || id.String() != form.ID {
			return types.NewApiError(i18n.T(ctx, "projects.invalid_id"), map[string]string{"id": i18n.T(ctx, "projects.invalid_id")})
		}
	}
	return validateProject(ctx, form.Name, form.Clients)
}

func validateProject(ctx context.Context, name string, clients []string) error {
	fields := make(map[string]string)
	if strings.TrimSpace(name) == "" {
		fields["name"] = i18n.T(ctx, "projects.name_required")
	} else if message := projectTextError(ctx, name, 3, types.ProjectNameMaxLength); message != "" {
		fields["name"] = message
	}
	for _, client := range clients {
		if strings.TrimSpace(client) == "" {
			fields["clients"] = i18n.T(ctx, "projects.clients_blank")
			break
		}
		if message := projectTextError(ctx, client, 1, types.ClientNameMaxLength); message != "" {
			fields["clients"] = message
			break
		}
	}
	if len(fields) > 0 {
		return types.NewApiError(i18n.T(ctx, "validation.failed"), fields)
	}
	return nil
}

func ValidateProjectPage(ctx context.Context, query types.GetProjectPageInput) error {
	fields := make(map[string]string)
	if query.Size < 1 || query.Size > 100 {
		fields["size"] = i18n.T(ctx, "projects.invalid_pagination")
	}
	if query.Page < 0 || (query.Size > 0 && query.Page > math.MaxInt64/query.Size) {
		fields["page"] = i18n.T(ctx, "projects.invalid_pagination")
	}
	if message := projectTextError(ctx, query.Search, 0, types.ProjectSearchMaxLength); message != "" {
		fields["search"] = message
	}
	if len(fields) > 0 {
		return types.NewApiError(i18n.T(ctx, "request.invalid_parameters"), fields)
	}
	return nil
}

func projectTextError(ctx context.Context, value string, minimum, maximum int) string {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return i18n.T(ctx, "validation.invalid")
	}
	length := utf8.RuneCountInString(strings.TrimSpace(value))
	if length < minimum {
		return i18n.Format(ctx, i18n.Message{ID: "validation.min_length", Data: map[string]any{"Count": minimum}, Count: &minimum})
	}
	// Bound the complete submitted value, including surrounding whitespace.
	if utf8.RuneCountInString(value) > maximum {
		return i18n.Format(ctx, i18n.Message{ID: "validation.max_length", Data: map[string]any{"Count": maximum}, Count: &maximum})
	}
	return ""
}
