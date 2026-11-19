package projects

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/lib/validation"
)

const (
	ProjectNameMaxLength   = 255
	ClientNameMaxLength    = 255
	ProjectSearchMaxLength = 1024
)

func validateProject(input ProjectInput) error {
	fields := make(map[string]validation.Violation)
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = validation.Violation{Key: "projects.name_required"}
	} else if violation := projectTextError(input.Name, 3, ProjectNameMaxLength); violation.Key != "" {
		fields["name"] = violation
	}
	for _, client := range input.Clients {
		if strings.TrimSpace(client) == "" {
			fields["clients"] = validation.Violation{Key: "projects.clients_blank"}
			break
		}
		if violation := projectTextError(client, 1, ClientNameMaxLength); violation.Key != "" {
			fields["clients"] = violation
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
		fields["size"] = validation.Violation{Key: "projects.invalid_pagination"}
	}
	if page < 0 || (size > 0 && page > math.MaxInt64/size) {
		fields["page"] = validation.Violation{Key: "projects.invalid_pagination"}
	}
	if violation := projectTextError(search, 0, ProjectSearchMaxLength); violation.Key != "" {
		fields["search"] = violation
	}
	if len(fields) > 0 {
		return PageQuery{}, &validation.Error{Key: "request.invalid_parameters", Fields: fields}
	}
	return PageQuery{search: search, page: page, size: size}, nil
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
