package projects

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/validation"
	"github.com/markojerkic/svarog/internal/server/types"
)

func validateProjectID(value string) error {
	if value != "" {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return &validation.Error{Key: "projects.invalid_id", Fields: map[string]validation.Violation{"id": {Key: "projects.invalid_id"}}}
		}
	}
	return nil
}

func validateProject(name string, clients []string) error {
	fields := make(map[string]validation.Violation)
	if strings.TrimSpace(name) == "" {
		fields["name"] = validation.Violation{Key: "projects.name_required"}
	} else if violation := projectTextError(name, 3, types.ProjectNameMaxLength); violation.Key != "" {
		fields["name"] = violation
	}
	for _, client := range clients {
		if strings.TrimSpace(client) == "" {
			fields["clients"] = validation.Violation{Key: "projects.clients_blank"}
			break
		}
		if violation := projectTextError(client, 1, types.ClientNameMaxLength); violation.Key != "" {
			fields["clients"] = violation
			break
		}
	}
	if len(fields) > 0 {
		return &validation.Error{Key: "validation.failed", Fields: fields}
	}
	return nil
}

func ValidateProjectPage(query types.GetProjectPageInput) error {
	fields := make(map[string]validation.Violation)
	if query.Size < 1 || query.Size > 100 {
		fields["size"] = validation.Violation{Key: "projects.invalid_pagination"}
	}
	if query.Page < 0 || (query.Size > 0 && query.Page > math.MaxInt64/query.Size) {
		fields["page"] = validation.Violation{Key: "projects.invalid_pagination"}
	}
	if violation := projectTextError(query.Search, 0, types.ProjectSearchMaxLength); violation.Key != "" {
		fields["search"] = violation
	}
	if len(fields) > 0 {
		return &validation.Error{Key: "request.invalid_parameters", Fields: fields}
	}
	return nil
}

func projectTextError(value string, minimum, maximum int) validation.Violation {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return validation.Violation{Key: "validation.invalid"}
	}
	length := utf8.RuneCountInString(strings.TrimSpace(value))
	if length < minimum {
		return validation.Violation{Key: "validation.min_length", Count: minimum}
	}
	// Bound the complete submitted value, including surrounding whitespace.
	if utf8.RuneCountInString(value) > maximum {
		return validation.Violation{Key: "validation.max_length", Count: maximum}
	}
	return validation.Violation{}
}
