package postgres_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/stretchr/testify/require"
)

func TestIsConstraintViolation(t *testing.T) {
	violation := &pgconn.PgError{
		Code:           postgres.SQLStateUniqueViolation,
		ConstraintName: "projects_name_key",
	}

	for _, test := range []struct {
		name       string
		err        error
		code       string
		constraint string
		matches    bool
	}{
		{
			name:       "matching violation",
			err:        violation,
			code:       postgres.SQLStateUniqueViolation,
			constraint: "projects_name_key",
			matches:    true,
		},
		{
			name:       "wrapped matching violation",
			err:        fmt.Errorf("create project: %w", violation),
			code:       postgres.SQLStateUniqueViolation,
			constraint: "projects_name_key",
			matches:    true,
		},
		{
			name:       "different SQLSTATE",
			err:        violation,
			code:       postgres.SQLStateForeignKeyViolation,
			constraint: "projects_name_key",
		},
		{
			name:       "different constraint",
			err:        violation,
			code:       postgres.SQLStateUniqueViolation,
			constraint: "projects_pkey",
		},
		{
			name:       "non-PostgreSQL error",
			err:        errors.New("connection unavailable"),
			code:       postgres.SQLStateUniqueViolation,
			constraint: "projects_name_key",
		},
		{
			name:       "no error",
			code:       postgres.SQLStateUniqueViolation,
			constraint: "projects_name_key",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			matches := postgres.IsConstraintViolation(test.err, test.code, test.constraint)

			require.Equal(t, test.matches, matches, "Only the requested SQLSTATE and constraint may match: %s", test.name)
		})
	}
}
