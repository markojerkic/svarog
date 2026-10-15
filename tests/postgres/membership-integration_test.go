package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestActivePostgresMembership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	q := sqlc.New(pool)
	sessionStore := auth.NewPostgresSessionStore(pool, []byte("test-session-secret-with-32-bytes!"))
	service := auth.NewPostgresAuthService(pool, sessionStore)
	first, err := q.CreateProject(ctx, sqlc.CreateProjectParams{ID: uuid.New(), Name: "First"})
	require.NoError(t, err, "create first project for active membership tests")
	second, err := q.CreateProject(ctx, sqlc.CreateProjectParams{ID: uuid.New(), Name: "Second"})
	require.NoError(t, err, "create second project for active membership tests")
	user, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username: "member", FirstName: "Original", LastName: "User", Role: "user",
		ProjectIDs: []string{first.ID.String(), first.ID.String()},
	})
	require.NoError(t, err, "atomically create user, token and duplicate-safe memberships")
	require.Equal(t, []string{first.ID.String()}, user.ProjectIDs, "creation returns actual deduplicated memberships")
	var originalToken []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT token_hash FROM login_tokens WHERE user_id = $1", user.ID).Scan(&originalToken), "successful user creation also persists login token")

	e := echo.New()
	e.Use(session.Middleware(sessionStore))
	e.GET("/me", func(c *echo.Context) error {
		current, err := service.GetCurrentUser(c)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, current)
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	sess, err := sessionStore.New(req, auth.SVAROG_SESSION)
	require.NoError(t, err, "create session for membership-aware current-user lookup")
	sess.Values["user_id"] = user.ID.String()
	rec := httptest.NewRecorder()
	require.NoError(t, sessionStore.Save(req, rec, sess), "persist authenticated member session")
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1, "authenticated member session issues one cookie")
	currentUser := func(t *testing.T) auth.LoggedInUser {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.AddCookie(cookies[0])
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "authenticated session resolves current user")
		var current auth.LoggedInUser
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &current), "decode membership-aware authenticated user")
		return current
	}
	checkAccess := func(t *testing.T, userID, projectID string, expected bool, scenario string) {
		t.Helper()
		access, err := service.UserHasAccessToProject(ctx, userID, projectID)
		require.NoError(t, err, "evaluate active project access: %s", scenario)
		require.Equal(t, expected, access, scenario)
	}
	requireProjectError := func(t *testing.T, err error, scenario string) {
		t.Helper()
		var apiError types.ApiError
		require.ErrorAs(t, err, &apiError, scenario)
		require.NotEmpty(t, apiError.Fields["projects"], "project assignment errors identify the projects field: %s", scenario)
	}

	t.Run("all user reads include actual memberships", func(t *testing.T) {
		byID, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err, "load member by ID for edit form")
		require.Equal(t, user, byID, "ID lookup includes persisted project memberships")
		byUsername, err := service.GetUserByUsername(ctx, user.Username)
		require.NoError(t, err, "load member by username")
		require.Equal(t, user, byUsername, "username lookup includes persisted project memberships")
		page, count, err := service.GetUserPage(ctx, types.GetUserPageInput{Size: 10, Search: "member"})
		require.NoError(t, err, "list membership-aware users")
		require.EqualValues(t, 1, count, "searched user page counts the member")
		require.Len(t, page, 1, "searched user page returns the member")
		require.Equal(t, user.ProjectIDs, page[0].ProjectIDs, "user page includes assigned project IDs")
		require.Empty(t, page[0].Password, "membership-aware user page still excludes password hashes")
		ids, err := service.GetProjectIDsForUser(ctx, user.ID.String())
		require.NoError(t, err, "list member's assigned project IDs")
		require.Equal(t, user.ProjectIDs, ids, "project ID lookup returns actual assignments")
		require.Equal(t, user.ProjectIDs, currentUser(t).ProjectIDs, "authenticated user context includes assignments")
	})

	t.Run("regular admin unknown and malformed access", func(t *testing.T) {
		require.NoError(t, service.CreateInitialAdminUser(ctx), "create admin for active access policy checks")
		admin, err := service.GetUserByUsername(ctx, "admin")
		require.NoError(t, err, "load admin without explicit project memberships")
		require.Empty(t, admin.ProjectIDs, "admin access does not synthesize project memberships")
		checkAccess(t, user.ID.String(), first.ID.String(), true, "regular member can access assigned project")
		checkAccess(t, user.ID.String(), second.ID.String(), false, "regular member cannot access unassigned project")
		checkAccess(t, admin.ID.String(), second.ID.String(), true, "admin can access existing project without membership")
		checkAccess(t, admin.ID.String(), uuid.NewString(), false, "admin cannot access a missing project")
		checkAccess(t, user.ID.String(), uuid.NewString(), false, "regular user cannot access a missing project")
		checkAccess(t, uuid.NewString(), first.ID.String(), false, "unknown user cannot access an existing project")
		checkAccess(t, "not-a-uuid", first.ID.String(), false, "malformed user identifier cannot grant access")
		checkAccess(t, admin.ID.String(), "not-a-uuid", false, "malformed project identifier is denied even for admin")
		_, err = service.GetProjectIDsForUser(ctx, uuid.NewString())
		require.EqualError(t, err, auth.ErrUserNotFound, "project ID lookup preserves missing-user error mapping")
	})

	t.Run("failed create leaves no user token or memberships", func(t *testing.T) {
		for _, projectID := range []string{"not-a-uuid", uuid.NewString()} {
			result, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
				Username: "rejected-member", FirstName: "Rejected", Role: "user",
				ProjectIDs: []string{first.ID.String(), projectID},
			})
			requireProjectError(t, err, "invalid or missing project rejects user creation")
			require.Equal(t, auth.User{}, result, "failed user creation returns no persisted user")
			_, err = q.GetUserByUsername(ctx, "rejected-member")
			require.ErrorIs(t, err, pgx.ErrNoRows, "failed creation rolls back all user profile writes")
		}
		var tokens, memberships int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM login_tokens").Scan(&tokens), "count tokens after rejected creates")
		require.EqualValues(t, 1, tokens, "failed creation does not leave orphan tokens")
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_projects").Scan(&memberships), "count memberships after rejected creates")
		require.EqualValues(t, 1, memberships, "failed creation does not leave partial assignments")
		input := authInput("compound-rollback")
		input.ProjectIDs = []uuid.UUID{first.ID, uuid.New()}
		_, err := store.CreateUserWithToken(ctx, pool, input)
		requireSQLState(t, err, "23503", "compound creation rejects missing project foreign key")
		_, err = q.GetUserByID(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows, "compound creation rolls back known user ID")
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM login_tokens WHERE user_id = $1", input.ID).Scan(&count), "count rolled-back user's tokens")
		require.Zero(t, count, "membership failure rolls back inserted login token")
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_projects WHERE user_id = $1", input.ID).Scan(&count), "count rolled-back user's memberships")
		require.Zero(t, count, "membership failure rolls back earlier valid assignment")
	})

	t.Run("failed update preserves profile token and memberships", func(t *testing.T) {
		before, err := q.GetUserByID(ctx, user.ID)
		require.NoError(t, err, "snapshot all profile fields before rejected update")
		for _, projectID := range []string{"not-a-uuid", uuid.NewString()} {
			result, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
				ID: user.ID.String(), Username: "rejected-rename", FirstName: "Rejected", LastName: "Changed", Role: "admin",
				ProjectIDs: []string{second.ID.String(), projectID},
			})
			requireProjectError(t, err, "invalid or missing project rejects profile and membership update")
			require.Equal(t, auth.User{}, result, "failed update returns no updated user")
			after, err := q.GetUserByID(ctx, user.ID)
			require.NoError(t, err, "reload profile after rejected membership update")
			require.Equal(t, before, after, "rejected update preserves profile, role, password and timestamps")
			ids, err := service.GetProjectIDsForUser(ctx, user.ID.String())
			require.NoError(t, err, "reload assignments after rejected update")
			require.Equal(t, user.ProjectIDs, ids, "failed update restores old memberships instead of leaving partial replacement")
		}
		var token []byte
		require.NoError(t, pool.QueryRow(ctx, "SELECT token_hash FROM login_tokens WHERE user_id = $1", user.ID).Scan(&token), "reload login token after rejected profile updates")
		require.Equal(t, originalToken, token, "rejected profile updates preserve login token")
		_, err = service.CreateOrUpdateUser(ctx, types.CreateUserForm{ID: uuid.NewString(), Username: "missing-user", Role: "user"})
		require.EqualError(t, err, auth.ErrUserNotFound, "compound profile update preserves missing-user error mapping")
	})

	t.Run("replace and clear assignments refresh access and auth context", func(t *testing.T) {
		updated, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Updated", LastName: "User", Role: "user",
			ProjectIDs: []string{second.ID.String(), second.ID.String()},
		})
		require.NoError(t, err, "replace profile and memberships atomically")
		require.Equal(t, "Updated", updated.FirstName, "compound update persists profile change")
		require.Equal(t, user.Password, updated.Password, "membership replacement preserves password hash")
		require.Equal(t, user.NeedsPasswordReset, updated.NeedsPasswordReset, "membership replacement preserves password reset requirement")
		require.Equal(t, []string{second.ID.String()}, updated.ProjectIDs, "replacement removes old project and deduplicates new project")
		checkAccess(t, user.ID.String(), first.ID.String(), false, "removed membership immediately revokes access")
		checkAccess(t, user.ID.String(), second.ID.String(), true, "new membership immediately grants access")
		require.Equal(t, updated.ProjectIDs, currentUser(t).ProjectIDs, "existing session sees replaced memberships without another login")
		cleared, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Updated", LastName: "User", Role: "user", ProjectIDs: []string{},
		})
		require.NoError(t, err, "empty project selection clears all memberships")
		require.Empty(t, cleared.ProjectIDs, "cleared user has no assigned projects")
		checkAccess(t, user.ID.String(), second.ID.String(), false, "empty replacement revokes final project access")
		require.Empty(t, currentUser(t).ProjectIDs, "existing auth context sees cleared project selection")
	})

	t.Run("concurrent compound updates keep profile and membership together", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, project := range []sqlc.Project{first, second} {
			go func() {
				<-start
				_, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
					ID: user.ID.String(), Username: user.Username, FirstName: project.Name, LastName: "User", Role: "user",
					ProjectIDs: []string{project.ID.String()},
				})
				results <- err
			}()
		}
		close(start)
		for range 2 {
			require.NoError(t, <-results, "each concurrent compound user update succeeds")
		}
		updated, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err, "reload user after concurrent profile and membership replacements")
		ids := map[string]string{first.Name: first.ID.String(), second.Name: second.ID.String()}
		require.Equal(t, []string{ids[updated.FirstName]}, updated.ProjectIDs, "winning profile corresponds to one complete winning membership set")
	})

	t.Run("delete user cascades memberships and denies access", func(t *testing.T) {
		c := e.NewContext(httptest.NewRequest(http.MethodDelete, "/users", nil), httptest.NewRecorder())
		require.NoError(t, service.DeleteUser(c, user.ID.String()), "delete user with active membership and session")
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_projects WHERE user_id = $1", user.ID).Scan(&count), "count memberships after active user deletion")
		require.Zero(t, count, "user deletion cascades every project membership")
		checkAccess(t, user.ID.String(), first.ID.String(), false, "deleted user cannot access first project")
		checkAccess(t, user.ID.String(), second.ID.String(), false, "deleted user cannot access second project")
	})
}
