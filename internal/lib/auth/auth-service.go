package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx echo.Context, username string, password string) error
	LoginWithToken(ctx echo.Context, token string) error
	Logout(ctx echo.Context) error
	ResetPassword(ctx context.Context, userId string, form types.ResetPasswordForm) error
	GenerateLoginToken(ctx context.Context, userId string) (string, error)
	DeleteUser(ctx echo.Context, id string) error
	GetCurrentUser(ctx echo.Context) (LoggedInUser, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserPage(ctx context.Context, query types.GetUserPageInput) ([]User, int64, error)
	CreateOrUpdateUser(ctx context.Context, form types.CreateUserForm) (User, error)
	UserHasAccessToProject(ctx context.Context, userId, projectId string) (bool, error)
	GetProjectIDsForUser(ctx context.Context, userId string) ([]string, error)
}

const SVAROG_SESSION = "svarog_session"
const (
	ErrUserNotFound     = "User not found"
	UserAlreadyExists   = "User already exists"
	LoginTokenNotValid  = "Login token not valid"
	PasswordsDoNotMatch = "Passwords do not match"
	loginTokenDuration  = 24 * time.Hour
)

type PostgresAuthService struct {
	pool         *pgxpool.Pool
	queries      *sqlc.Queries
	sessionStore *PostgresSessionStore
}

var _ AuthService = (*PostgresAuthService)(nil)

func NewPostgresAuthService(pool *pgxpool.Pool, sessionStore *PostgresSessionStore) *PostgresAuthService {
	return &PostgresAuthService{pool: pool, queries: sqlc.New(pool), sessionStore: sessionStore}
}

func (a *PostgresAuthService) Login(ctx echo.Context, username, password string) error {
	user, err := a.queries.GetUserByUsername(ctx.Request().Context(), username)
	if err != nil {
		return authError(err)
	}
	if !checkPasswordHash(password, user.PasswordHash) {
		return errors.New("Invalid password")
	}
	sess, err := a.sessionStore.New(ctx.Request(), SVAROG_SESSION)
	if err != nil {
		return err
	}
	sess.Values["user_id"] = user.ID.String()
	return a.sessionStore.Save(ctx.Request(), ctx.Response(), sess)
}

func (a *PostgresAuthService) LoginWithToken(ctx echo.Context, token string) error {
	c := ctx.Request().Context()
	sess, err := a.sessionStore.New(ctx.Request(), SVAROG_SESSION)
	if err != nil {
		return err
	}
	sessionID := uuid.New()
	sess.ID = sessionID.String()
	err = pgx.BeginFunc(c, a.pool, func(tx pgx.Tx) error {
		q := a.queries.WithTx(tx)
		userID, err := q.ConsumeLoginToken(c, tokenHash(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New(LoginTokenNotValid)
		}
		if err != nil {
			return err
		}
		if _, err := q.CreateSession(c, sqlc.CreateSessionParams{
			ID: sessionID, UserID: userID, ExpiresAt: time.Now().Add(sessionDuration),
		}); err != nil {
			return err
		}
		sess.Values["user_id"] = userID.String()
		return q.DeleteExpiredSessions(c)
	})
	if err != nil {
		return err
	}
	sess.IsNew = false
	return a.sessionStore.setSessionCookie(ctx.Response(), sess)
}

func (a *PostgresAuthService) Logout(ctx echo.Context) error {
	sess, err := session.Get(SVAROG_SESSION, ctx)
	if err != nil {
		return err
	}
	sess.Options.MaxAge = -1
	return a.sessionStore.Save(ctx.Request(), ctx.Response(), sess)
}

func (a *PostgresAuthService) GetCurrentUser(ctx echo.Context) (LoggedInUser, error) {
	sess, err := session.Get(SVAROG_SESSION, ctx)
	if err != nil {
		return LoggedInUser{}, err
	}
	userID, ok := sess.Values["user_id"].(string)
	if !ok || userID == "" {
		return LoggedInUser{}, errors.New(ErrUserNotFound)
	}
	user, err := a.GetUserByID(ctx.Request().Context(), userID)
	if err != nil {
		return LoggedInUser{}, err
	}
	return LoggedInUser{
		ID: user.ID.String(), Username: user.Username, Role: user.Role,
		NeedsPasswordReset: user.NeedsPasswordReset, ProjectIDs: []string{},
	}, nil
}

func (a *PostgresAuthService) GetUserByID(ctx context.Context, id string) (User, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return User{}, errors.New(ErrUserNotFound)
	}
	user, err := a.queries.GetUserByID(ctx, userID)
	return toUser(user), authError(err)
}

func (a *PostgresAuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	user, err := a.queries.GetUserByUsername(ctx, username)
	return toUser(user), authError(err)
}

func (a *PostgresAuthService) GetUserPage(ctx context.Context, query types.GetUserPageInput) ([]User, int64, error) {
	if query.Page < 0 || query.Size <= 0 || query.Size > 100 || query.Page > math.MaxInt32/query.Size {
		return nil, 0, errors.New("Invalid user pagination")
	}
	rows, err := a.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Search: query.Search, PageSize: int32(query.Size), PageOffset: int32(query.Page * query.Size),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := a.queries.CountUsers(ctx, query.Search)
	if err != nil {
		return nil, 0, err
	}
	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, User{
			ID: row.ID, Username: row.Username, FirstName: row.FirstName, LastName: row.LastName,
			Role: Role(row.Role), NeedsPasswordReset: row.NeedsPasswordReset, ProjectIDs: []string{},
		})
	}
	return users, count, nil
}

func (a *PostgresAuthService) CreateOrUpdateUser(ctx context.Context, form types.CreateUserForm) (User, error) {
	if len(form.ProjectIDs) > 0 {
		return User{}, types.NewApiError("Project assignment is unavailable until projects move to PostgreSQL", map[string]string{
			"projects": "Project assignment is temporarily unavailable",
		})
	}
	if form.ID != "" {
		id, err := uuid.Parse(form.ID)
		if err != nil {
			return User{}, errors.New(ErrUserNotFound)
		}
		user, err := a.queries.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
			ID: id, Username: form.Username, FirstName: form.FirstName, LastName: form.LastName, Role: form.Role,
		})
		return toUser(user), authError(err)
	}
	hash, err := hashPassword(generateRandomPassword())
	if err != nil {
		return User{}, err
	}
	token := generateLoginToken()
	user, err := persistence.CreateUserWithToken(ctx, a.pool, persistence.CreateUserWithTokenInput{
		ID: uuid.New(), Username: form.Username, FirstName: form.FirstName, LastName: form.LastName,
		Role: form.Role, PasswordHash: hash, NeedsPasswordReset: true,
		Token: persistence.LoginTokenInput{Hash: tokenHash(token), ExpiresAt: time.Now().Add(loginTokenDuration)},
	})
	return toUser(user), authError(err)
}

func (a *PostgresAuthService) GenerateLoginToken(ctx context.Context, userID string) (string, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return "", errors.New(ErrUserNotFound)
	}
	hash, err := hashPassword(generateRandomPassword())
	if err != nil {
		return "", err
	}
	token := generateLoginToken()
	err = persistence.ReplaceLoginToken(ctx, a.pool, persistence.ReplaceLoginTokenInput{
		UserID: id, PasswordHash: hash,
		Token: persistence.LoginTokenInput{Hash: tokenHash(token), ExpiresAt: time.Now().Add(loginTokenDuration)},
	})
	if err != nil {
		return "", authError(err)
	}
	return token, nil
}

func (a *PostgresAuthService) ResetPassword(ctx context.Context, userID string, form types.ResetPasswordForm) error {
	if form.Password != form.RepeatedPassword {
		return errors.New(PasswordsDoNotMatch)
	}
	if len(form.Password) < 8 || len(form.Password) > 72 {
		return errors.New("Password must contain between 8 and 72 bytes")
	}
	id, err := uuid.Parse(userID)
	if err != nil {
		return errors.New(ErrUserNotFound)
	}
	hash, err := hashPassword(form.Password)
	if err != nil {
		return err
	}
	err = pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		q := a.queries.WithTx(tx)
		if _, err := q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: id, PasswordHash: hash, NeedsPasswordReset: false}); err != nil {
			return err
		}
		return q.DeleteLoginTokensForUser(ctx, id)
	})
	return authError(err)
}

func (a *PostgresAuthService) DeleteUser(ctx echo.Context, id string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return errors.New(ErrUserNotFound)
	}
	_, err = a.queries.DeleteUser(ctx.Request().Context(), userID)
	return authError(err)
}

func (a *PostgresAuthService) UserHasAccessToProject(ctx context.Context, userID, projectID string) (bool, error) {
	user, err := a.GetUserByID(ctx, userID)
	return err == nil && user.Role == ADMIN, err
}

func (a *PostgresAuthService) GetProjectIDsForUser(ctx context.Context, userID string) ([]string, error) {
	if _, err := a.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	return []string{}, nil
}

func (a *PostgresAuthService) CreateInitialAdminUser(ctx context.Context) error {
	_, err := a.queries.GetUserByUsername(ctx, "admin")
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	hash, err := hashPassword("ADMINADMIN")
	if err != nil {
		return err
	}
	_, err = a.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID: uuid.New(), Username: "admin", FirstName: "Admin", LastName: "Admin",
		PasswordHash: hash, Role: string(ADMIN), NeedsPasswordReset: true,
	})
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "users_username_key" {
		return nil
	}
	return err
}

func toUser(user sqlc.User) User {
	return User{
		ID: user.ID, Username: user.Username, FirstName: user.FirstName, LastName: user.LastName,
		Password: user.PasswordHash, Role: Role(user.Role), NeedsPasswordReset: user.NeedsPasswordReset,
		ProjectIDs: []string{},
	}
}

func authError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New(ErrUserNotFound)
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "users_username_key" {
		return errors.New(UserAlreadyExists)
	}
	return err
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func generateRandomPassword() string {
	result, err := password.Generate(64, 10, 10, false, false)
	if err != nil {
		panic(fmt.Sprintf("generate random password: %v", err))
	}
	return result
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateLoginToken() string {
	return rand.Text()
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
