package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/validation"
	"github.com/markojerkic/svarog/internal/server/http/forms"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

func validationResponse(ctx context.Context, err error) (forms.ApiError, bool) {
	var invalid *validation.Error
	if !errors.As(err, &invalid) {
		return forms.ApiError{}, false
	}
	fields := make(map[string]string, len(invalid.Fields))
	for name, violation := range invalid.Fields {
		switch {
		case violation.Count != 0:
			count := violation.Count
			fields[name] = i18n.Format(ctx, i18n.Message{ID: violation.Key, Data: map[string]any{"Count": count}, Count: &count})
		case len(violation.Options) != 0:
			labels := make([]string, 0, len(violation.Options))
			for _, key := range violation.Options {
				labels = append(labels, i18n.T(ctx, key))
			}
			fields[name] = i18n.Format(ctx, i18n.Message{ID: violation.Key, Data: map[string]any{"Options": strings.Join(labels, ", ")}})
		default:
			fields[name] = i18n.T(ctx, violation.Key)
		}
	}
	return forms.NewApiError(i18n.T(ctx, invalid.Key), fields), true
}

func canonicalFormUUID(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil || id.String() != value {
		return uuid.Nil
	}
	return id
}
