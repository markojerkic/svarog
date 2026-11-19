package logstore

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type Store struct {
	queries *sqlc.Queries
	search  *sqlc.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{
		queries: sqlc.New(pool),
		search:  sqlc.New(searchQueryDB{DBTX: pool}),
	}
}
