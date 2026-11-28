package logstore

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type Store struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}
