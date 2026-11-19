package projects_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/stretchr/testify/require"
)

func TestProjectInputRejectedBeforeDatabaseAccess(t *testing.T) {
	service := projects.NewPostgresProjectsService(nil)
	ctx := context.Background()

	_, err := service.Create(ctx, projects.ProjectInput{Name: " "})

	require.Error(t, err, "blank project name must fail before database access")

	_, err = service.Create(ctx, projects.ProjectInput{Name: "Project", Clients: []string{" "}})

	require.Error(t, err, "blank client name must fail before database access")

	_, err = service.Update(ctx, uuid.New(), projects.ProjectInput{Name: "Project", Clients: []string{""}})

	require.Error(t, err, "updating with a blank client must fail before database access")

	_, err = service.Get(ctx, "not-a-uuid")

	require.ErrorIs(t, err, projects.ErrNotFound, "legacy project ID must not be accepted as a UUID")

	for _, query := range []struct {
		Search     string
		Page, Size int64
	}{
		{
			Page: -1,
			Size: 10,
		},
		{Size: 0},
		{Size: 101},
		{
			Page: 1 << 62,
			Size: 10,
		},
	} {
		_, err = projects.NewPageQuery(query.Search, query.Page, query.Size)

		require.Error(t, err, "invalid pagination %+v must fail before database access", query)
	}

	require.False(t, service.ProjectExists(ctx, "invalid", "*"), "wildcard credentials still require a valid project UUID")

	require.False(t, service.ProjectExists(ctx, uuid.NewString(), "client-name"), "client names must not be accepted as credential UUIDs")
}
