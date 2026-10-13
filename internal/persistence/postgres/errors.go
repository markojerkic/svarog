package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	SQLStateRestrictViolation   = "23001"
	SQLStateForeignKeyViolation = "23503"
	SQLStateUniqueViolation     = "23505"
)

func IsConstraintViolation(err error, code, constraintName string) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == code && pgError.ConstraintName == constraintName
}
