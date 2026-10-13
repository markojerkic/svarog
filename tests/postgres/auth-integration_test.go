package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestActivePostgresAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("svarog_test"), postgres.WithUsername("svarog"), postgres.WithPassword("svarog"),
		postgres.WithInitScripts(filepath.Join("..", "..", "internal", "persistence", "postgres", "migrations", "000001_initial.up.sql")),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err)
	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
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
	require.NoError(t, service.CreateInitialAdminUser(ctx))

	t.Run("initial admin and password login", func(t *testing.T) {
		require.NoError(t, service.CreateInitialAdminUser(ctx))
		admin, err := service.GetUserByUsername(ctx, "admin")
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, admin.ID)
		require.Equal(t, auth.ADMIN, admin.Role)
		require.True(t, admin.NeedsPasswordReset)
		require.NoError(t, service.CreateInitialAdminUser(ctx))
		again, err := service.GetUserByUsername(ctx, "admin")
		require.NoError(t, err)
		require.Equal(t, admin, again)
		count, err := queries.CountUsers(ctx, "admin")
		require.NoError(t, err)
		require.EqualValues(t, 1, count)

		rec, err := login("admin", "incorrect")
		require.Error(t, err)
		require.Empty(t, rec.Result().Cookies())
		rec, err = login("admin", "ADMINADMIN")
		require.NoError(t, err)
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].HttpOnly)
		require.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
		require.Empty(t, cookies[0].Domain)
		me := request(http.MethodGet, "/me", cookies)
		require.Equal(t, http.StatusOK, me.Code)
		var current auth.LoggedInUser
		require.NoError(t, json.Unmarshal(me.Body.Bytes(), &current))
		require.Equal(t, admin.ID.String(), current.ID)
		require.True(t, current.NeedsPasswordReset)
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", nil).Code)
		badCookie := &http.Cookie{Name: cookies[0].Name, Value: cookies[0].Value + "tampered"}
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", []*http.Cookie{badCookie}).Code)

		logout := request(http.MethodPost, "/logout", cookies)
		require.Equal(t, http.StatusOK, logout.Code)
		require.Less(t, logout.Result().Cookies()[0].MaxAge, 0)
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code)
	})

	t.Run("create update list and reject project assignment", func(t *testing.T) {
		user, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "testuser", FirstName: "Test", LastName: "User", Role: "user",
		})
		require.NoError(t, err)
		require.Empty(t, user.ProjectIDs)
		_, err = service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "testuser", FirstName: "Test", LastName: "User", Role: "user",
		})
		require.EqualError(t, err, auth.UserAlreadyExists)
		updated, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Changed", LastName: "User", Role: "user",
		})
		require.NoError(t, err)
		require.Equal(t, "Changed", updated.FirstName)
		require.Equal(t, user.Password, updated.Password)
		_, err = service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			ID: user.ID.String(), Username: user.Username, FirstName: "Rejected", Role: "admin", ProjectIDs: []string{"mongo-id"},
		})
		require.Error(t, err)
		again, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err)
		require.Equal(t, updated, again)
		users, count, err := service.GetUserPage(ctx, types.GetUserPageInput{Size: 10, Search: "TESTUSER"})
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
		require.Len(t, users, 1)
		require.Empty(t, users[0].Password)
		access, err := service.UserHasAccessToProject(ctx, user.ID.String(), "mongo-id")
		require.NoError(t, err)
		require.False(t, access)
		ids, err := service.GetProjectIDsForUser(ctx, user.ID.String())
		require.NoError(t, err)
		require.Empty(t, ids)
	})

	t.Run("hashed token login reset and session revocation", func(t *testing.T) {
		user, err := service.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username: "tokenuser", FirstName: "Token", LastName: "User", Role: "user",
		})
		require.NoError(t, err)
		firstToken, err := service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err)
		token, err := service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err)
		var storedHash []byte
		require.NoError(t, pool.QueryRow(ctx, "SELECT token_hash FROM login_tokens WHERE user_id = $1", user.ID).Scan(&storedHash))
		expectedHash := sha256.Sum256([]byte(token))
		require.Equal(t, expectedHash[:], storedHash)
		rec, err := tokenLogin(firstToken)
		require.EqualError(t, err, auth.LoginTokenNotValid)
		require.Empty(t, rec.Result().Cookies())
		rec, err = tokenLogin(token)
		require.NoError(t, err)
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, http.StatusOK, request(http.MethodGet, "/me", cookies).Code)
		rec, err = tokenLogin(token)
		require.EqualError(t, err, auth.LoginTokenNotValid)
		require.Empty(t, rec.Result().Cookies())
		require.EqualError(t, service.ResetPassword(ctx, user.ID.String(), types.ResetPasswordForm{
			Password: "newpassword", RepeatedPassword: "different",
		}), auth.PasswordsDoNotMatch)
		require.NoError(t, service.ResetPassword(ctx, user.ID.String(), types.ResetPasswordForm{
			Password: "newpassword", RepeatedPassword: "newpassword",
		}))
		updated, err := service.GetUserByID(ctx, user.ID.String())
		require.NoError(t, err)
		require.False(t, updated.NeedsPasswordReset)
		rec, err = login(user.Username, "newpassword")
		require.NoError(t, err)
		passwordCookies := rec.Result().Cookies()
		token, err = service.GenerateLoginToken(ctx, user.ID.String())
		require.NoError(t, err)
		_, err = login(user.Username, "newpassword")
		require.Error(t, err)
		_, err = pool.Exec(ctx, `UPDATE login_tokens SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE user_id = $1`, user.ID)
		require.NoError(t, err)
		_, err = tokenLogin(token)
		require.EqualError(t, err, auth.LoginTokenNotValid)
		deleteContext := e.NewContext(httptest.NewRequest(http.MethodDelete, "/users", nil), httptest.NewRecorder())
		require.NoError(t, service.DeleteUser(deleteContext, user.ID.String()))
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code)
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", passwordCookies).Code)
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id = $1", user.ID).Scan(&count))
		require.Zero(t, count)
	})

	t.Run("failed session creation rolls back token consumption", func(t *testing.T) {
		userID := uuid.New()
		_, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
			ID: userID, Username: "rollback-user", FirstName: "Rollback", LastName: "User",
			PasswordHash: "unused-fixture-hash", Role: "user", NeedsPasswordReset: true,
		})
		require.NoError(t, err)
		token := "rollback-token-fixture"
		hash := sha256.Sum256([]byte(token))
		require.NoError(t, queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			UserID: userID, TokenHash: hash[:], ExpiresAt: time.Now().Add(time.Hour),
		}))
		_, err = pool.Exec(ctx, `
			CREATE FUNCTION fail_session_insert() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'session insert failure'; END; $$;
			CREATE TRIGGER fail_session_insert BEFORE INSERT ON sessions
			FOR EACH ROW EXECUTE FUNCTION fail_session_insert();`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := pool.Exec(ctx, "DROP TRIGGER IF EXISTS fail_session_insert ON sessions; DROP FUNCTION IF EXISTS fail_session_insert()")
			require.NoError(t, err)
		})
		rec, err := tokenLogin(token)
		require.Error(t, err)
		require.Empty(t, rec.Result().Cookies())
		_, err = pool.Exec(ctx, "DROP TRIGGER fail_session_insert ON sessions")
		require.NoError(t, err)
		rec, err = tokenLogin(token)
		require.NoError(t, err)
		require.Len(t, rec.Result().Cookies(), 1)
	})

	t.Run("session expiry cannot be revived and database failures do not set cookies", func(t *testing.T) {
		rec, err := login("admin", "ADMINADMIN")
		require.NoError(t, err)
		cookies := rec.Result().Cookies()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookies[0])
		sess, err := store.Get(req, auth.SVAROG_SESSION)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE sessions SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE id = $1`, sess.ID)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, request(http.MethodGet, "/me", cookies).Code)
		out := httptest.NewRecorder()
		require.ErrorIs(t, store.Save(req, out, sess), pgx.ErrNoRows)
		require.Empty(t, out.Result().Cookies())
		fresh, err := store.New(req, auth.SVAROG_SESSION)
		require.NoError(t, err)
		fresh.Values["user_id"] = uuid.NewString()
		out = httptest.NewRecorder()
		requireSQLState(t, store.Save(req, out, fresh), "23503")
		require.Empty(t, out.Result().Cookies())
	})
}
