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
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectsService := projects.NewPostgresProjectsService(pool)
	project, err := projectsService.CreateProject(ctx, "ArchivePolicies", []string{"api", "worker"})
	require.NoError(t, err, "create project and clients for archive policies")
	other, err := projectsService.CreateProject(ctx, "ArchivePoliciesOther", []string{"api"})
	require.NoError(t, err, "create a second project")
	var service archive.Archiver = archive.NewArchiveService(pool, archive.NewDiskStore(t.TempDir()))
	scope := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")}

	t.Run("missing policy is disabled", func(t *testing.T) {
		policy, err := service.GetPolicy(ctx, scope)
		require.NoError(t, err, "a client without a policy remains readable")
		require.Equal(t, archive.Policy{}, policy, "a missing rule is a disabled policy")
	})

	t.Run("set creates updates and repeats", func(t *testing.T) {
		for _, weeks := range []int{10, 20, 20} {
			want := archive.Policy{Enabled: true, ArchiveAfterWeeks: weeks}
			require.NoError(t, service.SetPolicy(ctx, scope, want), "set replaces the desired policy idempotently")
			got, err := service.GetPolicy(ctx, scope)
			require.NoError(t, err, "read the saved policy")
			require.Equal(t, want, got, "the policy reflects the latest desired state")
		}
	})

	t.Run("scope validation covers every operation", func(t *testing.T) {
		want := archive.Policy{Enabled: true, ArchiveAfterWeeks: 20}
		require.NoError(t, service.SetPolicy(ctx, scope, want), "set the initial policy for scope validation independently")
		for _, invalid := range []archive.Scope{
			{ProjectID: other.ID, ClientID: scope.ClientID},
			{ProjectID: project.ID, ClientID: uuid.New()},
		} {
			_, err := service.GetPolicy(ctx, invalid)
			require.ErrorIs(t, err, archive.ErrClientNotFound, "a missing or wrong-project client cannot be read")
			require.ErrorIs(t, service.SetPolicy(ctx, invalid, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), archive.ErrClientNotFound, "a wrong scope cannot enable or update a policy")
			require.ErrorIs(t, service.SetPolicy(ctx, invalid, archive.Policy{}), archive.ErrClientNotFound, "a wrong scope cannot disable a policy")
			_, err = service.Create(ctx, invalid)
			require.ErrorIs(t, err, archive.ErrClientNotFound, "a wrong scope cannot create an archive")
			_, err = service.List(ctx, invalid, archive.ListQuery{})
			require.ErrorIs(t, err, archive.ErrClientNotFound, "a wrong scope cannot list archives")
			_, err = service.Open(ctx, invalid, uuid.New())
			require.ErrorIs(t, err, archive.ErrClientNotFound, "a wrong scope cannot open archives")
		}
		_, err := service.GetPolicy(ctx, archive.Scope{})
		require.ErrorIs(t, err, archive.ErrInvalidInput, "empty UUIDs are invalid input")
		policy, err := service.GetPolicy(ctx, scope)
		require.NoError(t, err, "read the original policy after wrong-scope mutations")
		require.Equal(t, want, policy, "rejected mutations do not alter another project's policy")
	})

	t.Run("weeks are bounded by cutoff arithmetic", func(t *testing.T) {
		for _, weeks := range []int{0, -1, archive.MaxArchiveAfterWeeks + 1} {
			err := service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: weeks})
			require.ErrorIs(t, err, archive.ErrInvalidInput, "unsupported week count %d is rejected", weeks)
		}
		require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: archive.MaxArchiveAfterWeeks}), "the largest safe week count is accepted")
	})

	t.Run("disable is idempotent and ignores weeks", func(t *testing.T) {
		require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "enable the initial policy before testing disabling")
		for range 2 {
			require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{ArchiveAfterWeeks: -1}), "disabling is idempotent and ignores the week count")
		}
		policy, err := service.GetPolicy(ctx, scope)
		require.NoError(t, err, "a disabled policy remains readable")
		require.Equal(t, archive.Policy{}, policy, "disabled policies have a canonical zero value")
	})

	t.Run("inactive client remains supported", func(t *testing.T) {
		inactive := archive.Scope{ProjectID: other.ID, ClientID: clientByName(t, other, "api")}
		want := archive.Policy{Enabled: true, ArchiveAfterWeeks: 6}
		require.NoError(t, service.SetPolicy(ctx, inactive, want), "enable archiving before deactivation")
		_, err := projectsService.UpdateProject(ctx, other.ID, other.Name, nil)
		require.NoError(t, err, "deactivate the client without removing history")
		got, err := service.GetPolicy(ctx, inactive)
		require.NoError(t, err, "inactive clients retain their policy")
		require.Equal(t, want, got, "deactivation preserves the archive policy")
	})

	t.Run("missing configuration only prevents enabling", func(t *testing.T) {
		unconfigured := archive.NewArchiveService(pool, nil)
		require.ErrorIs(t, unconfigured.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), archive.ErrNotConfigured, "enabling requires a configured directory")
		require.NoError(t, unconfigured.SetPolicy(ctx, scope, archive.Policy{}), "disabling does not require storage configuration")
		result, err := unconfigured.Create(ctx, scope)
		require.NoError(t, err, "an unconfigured but disabled client is an expected skip")
		require.Equal(t, archive.SkippedDisabled, result.Outcome, "disabled clients do not require storage")
		require.Nil(t, result.Archive, "skipped results have no archive")
	})
}
