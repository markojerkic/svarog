package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestActivePostgresAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	queries := sqlc.New(pool)
	store := auth.NewPostgresSessionStore(pool, []byte("test-session-secret-with-32-bytes!"))
	service := auth.NewPostgresAuthService(pool, store)
	e := echo.New()
	e.Use(session.Middleware(store))
	e.GET("/me", func(c echo.Context) error {
		user, err := service.GetCurrentUser(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized)
		}
		return c.JSON(http.StatusOK, user)
	})
	e.POST("/logout", func(c echo.Context) error {
		if err := service.Logout(c); err != nil {
			return err
		}
		return c.NoContent(http.StatusOK)
	})
	request := func(method, path string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	login := func(username, password string) (*httptest.ResponseRecorder, error) {
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodPost, "/login", nil), rec)
		return rec, service.Login(c, username, password)
	}
	tokenLogin := func(token string) (*httptest.ResponseRecorder, error) {
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/login", nil), rec)
		return rec, service.LoginWithToken(c, token)
	}
	require.NoError(t, service.CreateInitialAdminUser(ctx), "create initial admin for authentication tests")

	t.Run("initial admin and password login", func(t *testing.T) {
		require.NoError(t, service.CreateInitialAdminUser(ctx), "initial admin creation is safe to repeat")
		admin, err := service.GetUserByUsername(ctx, "admin")
		require.NoError(t, err, "look up initial admin")
		require.NotEqual(t, uuid.Nil, admin.ID, "initial admin has a generated ID")
		require.Equal(t, auth.ADMIN, admin.Role, "initial admin receives admin role")
		require.True(t, admin.NeedsPasswordReset, "initial admin must reset default password")
		require.NoError(t, service.CreateInitialAdminUser(ctx), "repeat admin initialization without changing existing user")
		again, err := service.GetUserByUsername(ctx, "admin")
		require.NoError(t, err, "reload admin after repeated initialization")
		require.Equal(t, admin, again, "repeated initialization preserves admin fields")
		count, err := queries.CountUsers(ctx, "admin")
		require.NoError(t, err, "count initialized admins")
		require.EqualValues(t, 1, count, "initialization does not duplicate admin")

		rec, err := login("admin", "incorrect")
		require.Error(t, err, "incorrect password rejects login")
		require.Empty(t, rec.Result().Cookies(), "rejected password login must not set session cookies")
		rec, err = login("admin", "ADMINADMIN")
		require.NoError(t, err, "default admin password authenticates")
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1, "successful login sets one session cookie")
		require.True(t, cookies[0].HttpOnly, "session cookie is inaccessible to JavaScript")
		require.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite, "session cookie uses strict same-site policy")
		require.Empty(t, cookies[0].Domain, "session cookie is host-only")
		me := request(http.MethodGet, "/me", cookies)
		require.Equal(t, http.StatusOK, me.Code, "valid session grants current-user access")
		var current auth.LoggedInUser
		require.NoError(t, json.Unmarshal(me.Body.Bytes(), &current), "decode authenticated current user")
		require.Equal(t, admin.ID.String(), current.ID, "session resolves to logged-in admin")
		require.True(t, current.NeedsPasswordReset, "current user retains password reset requirement")
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", nil).Code, "request without session is unauthorized")
		badCookie := &http.Cookie{Name: cookies[0].Name, Value: cookies[0].Value + "tampered"}
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", []*http.Cookie{badCookie}).Code, "tampered session cookie is unauthorized")

		logout := request(http.MethodPost, "/logout", cookies)
		require.Equal(t, http.StatusOK, logout.Code, "logout succeeds for authenticated session")
		require.Less(t, logout.Result().Cookies()[0].MaxAge, 0, "logout expires browser session cookie")
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code, "logout revokes session even when original cookie is replayed")
	})

	t.Run("create update list and reject invalid project assignment", func(t *testing.T) {
		user, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "testuser", FirstName: "Test", LastName: "User", Role: "user",
		})
		require.NoError(t, err, "create regular user without project assignments")
		require.Empty(t, user.ProjectIDs, "new user has no assigned projects")
		_, err = service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "testuser", FirstName: "Test", LastName: "User", Role: "user",
		})
		require.EqualError(t, err, auth.UserAlreadyExists, "duplicate username maps to user-already-exists error")
		updated, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Changed", LastName: "User", Role: "user",
		})
		require.NoError(t, err, "update existing user profile")
		require.Equal(t, "Changed", updated.FirstName, "profile update persists first name")
		require.Equal(t, user.Password, updated.Password, "profile update preserves password")
		_, err = service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Rejected", Role: "admin", ProjectIDs: []string{"not-a-uuid"},
		})
		var apiError types.ApiError
		require.ErrorAs(t, err, &apiError, "reject malformed project UUID before profile update")
		require.NotEmpty(t, apiError.Fields["projects"], "invalid project UUID reports a projects field error")
		again, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err, "reload user after rejected assignment")
		require.Equal(t, updated, again, "rejected assignment leaves profile unchanged")
		users, count, err := service.GetUserPage(ctx, types.GetUserPageInput{Size: 10, Search: "TESTUSER"})
		require.NoError(t, err, "search user page case-insensitively")
		require.EqualValues(t, 1, count, "user page count matches searched username")
		require.Len(t, users, 1, "user page returns searched user")
		require.Empty(t, users[0].Password, "user page must not expose password hashes")
		access, err := service.UserHasAccessToProject(ctx, user.ID.String(), "not-a-uuid")
		require.NoError(t, err, "deny malformed project ID without a database error")
		require.False(t, access, "user has no access to malformed project ID")
		ids, err := service.GetProjectIDsForUser(ctx, user.ID.String())
		require.NoError(t, err, "list projects after rejected assignment")
		require.Empty(t, ids, "rejected assignment creates no memberships")
	})

	t.Run("hashed token login reset and session revocation", func(t *testing.T) {
		user, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "tokenuser", FirstName: "Token", LastName: "User", Role: "user",
		})
		require.NoError(t, err, "create user for token authentication")
		firstToken, err := service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err, "generate initial login token")
		token, err := service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err, "generate replacement login token")
		var storedHash []byte
		require.NoError(t, pool.QueryRow(ctx, "SELECT token_hash FROM login_tokens WHERE user_id = $1", user.ID).Scan(&storedHash), "read persisted login token hash")
		expectedHash := sha256.Sum256([]byte(token))
		require.Equal(t, expectedHash[:], storedHash, "persist SHA-256 token hash rather than raw token")
		rec, err := tokenLogin(firstToken)
		require.EqualError(t, err, auth.LoginTokenNotValid, "replacement invalidates initial token")
		require.Empty(t, rec.Result().Cookies(), "invalidated token must not set session cookie")
		rec, err = tokenLogin(token)
		require.NoError(t, err, "replacement token authenticates successfully")
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1, "token login sets one session cookie")
		require.Equal(t, http.StatusOK, request(http.MethodGet, "/me", cookies).Code, "token session grants current-user access")
		rec, err = tokenLogin(token)
		require.EqualError(t, err, auth.LoginTokenNotValid, "consumed token cannot authenticate again")
		require.Empty(t, rec.Result().Cookies(), "replayed token must not set session cookie")
		require.EqualError(t, service.ResetPassword(ctx, user.ID.String(), types.ResetPasswordForm{
			Password: "newpassword", RepeatedPassword: "different",
		}), auth.PasswordsDoNotMatch, "reject mismatched password confirmation")
		require.NoError(t, service.ResetPassword(ctx, user.ID.String(), types.ResetPasswordForm{
			Password: "newpassword", RepeatedPassword: "newpassword",
		}), "reset password with matching confirmation")
		updated, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err, "reload user after password reset")
		require.False(t, updated.NeedsPasswordReset, "successful reset clears password reset requirement")
		rec, err = login(user.Username, "newpassword")
		require.NoError(t, err, "new password authenticates after reset")
		passwordCookies := rec.Result().Cookies()
		token, err = service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err, "generate token after password login")
		_, err = login(user.Username, "newpassword")
		require.Error(t, err, "token generation invalidates previous password")
		_, err = pool.Exec(ctx, `UPDATE login_tokens SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE user_id = $1`, user.ID)
		require.NoError(t, err, "expire generated login token")
		_, err = tokenLogin(token)
		require.EqualError(t, err, auth.LoginTokenNotValid, "expired login token is rejected")
		deleteContext := e.NewContext(httptest.NewRequest(http.MethodDelete, "/users", nil), httptest.NewRecorder())
		require.NoError(t, service.DeleteUser(deleteContext, user.ID.String()), "delete user with token and password sessions")
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code, "user deletion revokes token session")
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", passwordCookies).Code, "user deletion revokes password session")
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id = $1", user.ID).Scan(&count), "count sessions after user deletion")
		require.Zero(t, count, "user deletion removes all persisted sessions")
	})

	t.Run("failed session creation rolls back token consumption", func(t *testing.T) {
		userID := uuid.New()
		_, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
			ID: userID, Username: "rollback-user", FirstName: "Rollback", LastName: "User",
			PasswordHash: "unused-fixture-hash", Role: "user", NeedsPasswordReset: true,
		})
		require.NoError(t, err, "create fixture user for session rollback test")
		token := "rollback-token-fixture"
		hash := sha256.Sum256([]byte(token))
		require.NoError(t, queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			UserID: userID, TokenHash: hash[:], ExpiresAt: time.Now().Add(time.Hour),
		}), "persist fixture token for session rollback test")
		_, err = pool.Exec(ctx, `
			CREATE FUNCTION fail_session_insert() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'session insert failure'; END; $$;
			CREATE TRIGGER fail_session_insert BEFORE INSERT ON sessions
			FOR EACH ROW EXECUTE FUNCTION fail_session_insert();`)
		require.NoError(t, err, "install trigger that forces session insertion failure")
		t.Cleanup(func() {
			_, err := pool.Exec(ctx, "DROP TRIGGER IF EXISTS fail_session_insert ON sessions; DROP FUNCTION IF EXISTS fail_session_insert()")
			require.NoError(t, err, "clean up session failure trigger and function")
		})
		rec, err := tokenLogin(token)
		require.Error(t, err, "session insertion failure rejects token login")
		require.Empty(t, rec.Result().Cookies(), "failed session creation must not set cookie")
		_, err = pool.Exec(ctx, "DROP TRIGGER fail_session_insert ON sessions")
		require.NoError(t, err, "remove session insertion failure trigger")
		rec, err = tokenLogin(token)
		require.NoError(t, err, "failed session creation rolls back consumption so same token can log in")
		require.Len(t, rec.Result().Cookies(), 1, "retried token login creates session cookie")
	})

	t.Run("session expiry cannot be revived and database failures do not set cookies", func(t *testing.T) {
		rec, err := login("admin", "ADMINADMIN")
		require.NoError(t, err, "log in admin for session expiry test")
		cookies := rec.Result().Cookies()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookies[0])
		sess, err := store.Get(req, auth.SVAROG_SESSION)
		require.NoError(t, err, "load authenticated session from cookie")
		_, err = pool.Exec(ctx, `UPDATE sessions SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE id = $1`, sess.ID)
		require.NoError(t, err, "expire persisted session")
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code, "expired session cannot access current user")
		out := httptest.NewRecorder()
		require.ErrorIs(t, store.Save(req, out, sess), pgx.ErrNoRows, "saving expired session cannot revive it")
		require.Empty(t, out.Result().Cookies(), "failed expired-session save must not set cookie")
		fresh, err := store.New(req, auth.SVAROG_SESSION)
		require.NoError(t, err, "create fresh session for missing-user failure test")
		fresh.Values["user_id"] = uuid.NewString()
		out = httptest.NewRecorder()
		requireSQLState(t, store.Save(req, out, fresh), "23503", "session foreign key rejects missing parent user")
		require.Empty(t, out.Result().Cookies(), "database failure must not set session cookie")
	})
}
