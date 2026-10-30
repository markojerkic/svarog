package postgres_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrationsCreateTables(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("svarog_test"),
		postgres.WithUsername("svarog"),
		postgres.WithPassword("svarog"),
		postgres.WithOrderedInitScripts(testutils.PostgresMigrations(t)...),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start PostgreSQL with all migrations")

	exitCode, output, err := container.Exec(ctx, []string{
		"psql", "-X", "-U", "svarog", "-d", "svarog_test", "-At", "-v", "ON_ERROR_STOP=1", "-c",
		"SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename",
	}, exec.Multiplexed())
	require.NoError(t, err, "run query listing public tables")
	result, err := io.ReadAll(output)
	require.NoError(t, err, "read public table query output")
	require.Zero(t, exitCode, "public table query must succeed: %s", result)
	require.Equal(t, []string{
		"archive_policies",
		"archives",
		"clients",
		"login_tokens",
		"logs",
		"projects",
		"sessions",
		"user_projects",
		"users",
	}, strings.Fields(string(result)), "migrations create exactly the expected public tables")
}
