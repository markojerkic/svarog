package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestArchivePolicies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "ArchivePolicies", []string{"api", "worker"})
	require.NoError(t, err, "create project and clients for archive policies")
	other, err := projectService.CreateProject(ctx, "ArchivePoliciesOther", []string{"api"})
	require.NoError(t, err, "create a second project")
	service := archive.NewArchiveService(nil, nil, nil, nil, nil, pool)
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	projectID := project.ID.String()
	clientID := apiID.String()

	t.Run("create and get", func(t *testing.T) {
		require.NoError(t, service.CreateSetting(ctx, projectID, clientID, 10), "create a policy for a client in the project")
		setting, err := service.GetSettings(ctx, projectID, clientID)
		require.NoError(t, err, "read the created policy")
		require.Equal(t, clientID, setting.ClientID, "policy id is the client id")
		require.Equal(t, projectID, setting.ProjectID, "policy reports the owning project")
		require.Equal(t, 10, setting.ArhiveAfterWeeks, "policy stores the requested week count")
		require.False(t, setting.UpdatedAt.IsZero(), "policy records an update time")
	})

	t.Run("duplicate and wrong scope", func(t *testing.T) {
		require.ErrorIs(t, service.CreateSetting(ctx, projectID, clientID, 12), archive.ErrArchivePolicyExists, "a client can have only one policy")
		require.ErrorIs(t, service.CreateSetting(ctx, other.ID.String(), clientID, 10), archive.ErrArchivePolicyNotFound, "a client from another project is not writable")
		_, err := service.GetSettings(ctx, other.ID.String(), clientID)
		require.ErrorIs(t, err, archive.ErrArchivePolicyNotFound, "a client from another project is not readable")
		require.ErrorIs(t, service.CreateSetting(ctx, projectID, uuid.NewString(), 10), archive.ErrArchivePolicyNotFound, "a missing client does not get a policy")
		require.Error(t, service.CreateSetting(ctx, projectID, "marko", 10), "an invalid client id is rejected")
		require.Error(t, service.CreateSetting(ctx, projectID, clientID, 0), "zero weeks is rejected before the query")
		require.Error(t, service.CreateSetting(ctx, projectID, workerID.String(), -1), "negative weeks is rejected before the query")
	})

	t.Run("update and delete only the policy", func(t *testing.T) {
		require.NoError(t, service.UpdateSetting(ctx, clientID, 20), "update the policy by client id")
		setting, err := service.GetSettings(ctx, projectID, clientID)
		require.NoError(t, err, "read the updated policy")
		require.Equal(t, 20, setting.ArhiveAfterWeeks, "update replaces the week count")
		require.ErrorIs(t, service.UpdateSetting(ctx, uuid.NewString(), 20), archive.ErrArchivePolicyNotFound, "updating a missing policy fails")

		archiveID := uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO archives (id, client_id, storage_backend, object_key, file_name, cutoff_at)
			VALUES ($1, $2, 'disk', 'policies/test.zip', 'test.zip', now())`, archiveID, apiID)
		require.NoError(t, err, "insert an archive row that must survive policy deletion")
		require.NoError(t, service.DeleteSetting(ctx, clientID), "delete only the policy")
		_, err = service.GetSettings(ctx, projectID, clientID)
		require.ErrorIs(t, err, archive.ErrArchivePolicyNotFound, "deleted policy is no longer readable")
		var archives int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM archives WHERE id = $1", archiveID).Scan(&archives), "count the archive after policy deletion")
		require.Equal(t, 1, archives, "deleting a policy does not delete archive rows")
		require.ErrorIs(t, service.DeleteSetting(ctx, clientID), archive.ErrArchivePolicyNotFound, "deleting a missing policy fails")
	})

	t.Run("inactive client stays readable", func(t *testing.T) {
		inactiveClient := clientByName(t, other, "api")
		require.NoError(t, service.CreateSetting(ctx, other.ID.String(), inactiveClient.String(), 6), "create a policy before deactivating the client")
		_, err := projectService.UpdateProject(ctx, other.ID, other.Name, nil)
		require.NoError(t, err, "deactivate the client while retaining its row")
		setting, err := service.GetSettings(ctx, other.ID.String(), inactiveClient.String())
		require.NoError(t, err, "an inactive client policy stays readable")
		require.Equal(t, 6, setting.ArhiveAfterWeeks, "deactivation does not change the stored week count")
	})
}
