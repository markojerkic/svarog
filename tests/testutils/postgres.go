package testutils

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func NewPostgresPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	container, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("svarog_test"),
		postgres.WithUsername("svarog"),
		postgres.WithPassword("svarog"),
		postgres.WithInitScripts(filepath.Join("..", "..", "internal", "persistence", "postgres", "migrations", "000001_initial.up.sql")),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start PostgreSQL 18 container with initial migration")
	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get PostgreSQL connection string")
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err, "create PostgreSQL connection pool")
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx), "connect to migrated PostgreSQL test database")
	return pool
}
