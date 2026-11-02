package testutils

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/migrations"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func NewPostgresPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	container, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("svarog_test"),
		postgres.WithUsername("svarog"),
		postgres.WithPassword("svarog"),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start PostgreSQL 18 container")
	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get PostgreSQL connection string")
	require.NoError(t, migrations.Up(ctx, connectionString), "apply PostgreSQL migrations through Goose")
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err, "create PostgreSQL connection pool")
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx), "connect to migrated PostgreSQL test database")
	return pool
}
