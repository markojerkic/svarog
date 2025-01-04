package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx echo.Context, username string, password string) error
	Register(ctx echo.Context, username string, password string) error
	GetCurrentUser(ctx echo.Context) (LoggedInUser, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
}

type MongoAuthService struct {
	userCollection *mongo.Collection
	sessionStore   sessions.Store
}

const SVAROG_SESSION = "svarog_session"
const (
	ErrUserNotFound = "User not found"
) // Error codes

// GetUserByID implements AuthService.
func (self *MongoAuthService) GetUserByID(ctx context.Context, id string) (User, error) {
	var user User
	err := self.userCollection.FindOne(ctx, bson.M{
		"_id": id,
	}).Decode(&user)

	return user, err
}

// GetUserByUsername implements AuthService.
func (self *MongoAuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := self.userCollection.FindOne(ctx, bson.M{
		"username": username,
	}).Decode(&user)

	return user, err
}

// Register implements AuthService.
func (m *MongoAuthService) Register(ctx echo.Context, username string, password string) error {
	// Check if user already exists
	existingUserResult := m.userCollection.FindOne(ctx.Request().Context(), bson.M{
		"username": username,
	})
	if existingUserResult.Err() == nil {
		return fmt.Errorf("User %s already exists", username)
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	user, err := m.userCollection.InsertOne(ctx.Request().Context(), User{
		Username: username,
		Password: hashedPassword,
		Role:     USER,
	})

	userId, ok := user.InsertedID.(primitive.ObjectID)

	if !ok {
		return errors.New("Failed to get user ID")
	}

	return m.createSession(ctx, userId.Hex())
}

// GetCurrentUser implements AuthService.
func (m *MongoAuthService) GetCurrentUser(ctx echo.Context) (LoggedInUser, error) {
	session, err := m.sessionStore.Get(ctx.Request(), SVAROG_SESSION)
	if err != nil {
		return LoggedInUser{}, err
	}

	userId, ok := session.Values["user_id"].(string)
	slog.Info("User ID", slog.Any("user_id", userId))

	if !ok {
		return LoggedInUser{}, errors.New(ErrUserNotFound)
	}

	user, err := m.GetUserByID(ctx.Request().Context(), userId)

	if err != nil {
		return LoggedInUser{}, errors.Join(errors.New(ErrUserNotFound), err)
	}

	return LoggedInUser{
		ID:       user.ID.Hex(),
		Username: user.Username,
		Role:     user.Role,
	}, nil

}

// Login implements AuthService.
func (m *MongoAuthService) Login(ctx echo.Context, username string, password string) error {
	user, err := m.GetUserByUsername(ctx.Request().Context(), username)
	if err != nil {
		return err
	}

	passwordOk := checkPasswordHash(password, user.Password)

	if !passwordOk {
		return errors.New("Invalid password")
	}

	return m.createSession(ctx, user.ID.Hex())
}

func (self *MongoAuthService) createSession(ctx echo.Context, userID string) error {
	session, err := self.sessionStore.New(ctx.Request(), SVAROG_SESSION)
	if err != nil {
		return errors.Join(errors.New("Error creating session"), err)
	}
	session.Values["user_id"] = userID

	err = session.Save(ctx.Request(), ctx.Response())
	if err != nil {
		return errors.Join(errors.New("Error writting session to response"), err)
	}

	return nil
}

func (m *MongoAuthService) CreateInitialAdminUser(ctx context.Context) error {
	// Check if admin user already exists
	existingUserResult := m.userCollection.FindOne(ctx, bson.M{
		"username": "admin",
	})
	if existingUserResult.Err() == nil {
		slog.Info("Admin user already exists, not creating")
		return nil
	}

	hashedPassword, err := hashPassword("ADMINADMIN")
	if err != nil {
		return err
	}

	_, err = m.userCollection.InsertOne(ctx, User{
		Username: "admin",
		Password: hashedPassword,
		Role:     ADMIN,
	})
	return err
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

var _ AuthService = &MongoAuthService{}

func NewMongoAuthService(userCollection *mongo.Collection, sessionStore sessions.Store) *MongoAuthService {
	return &MongoAuthService{
		userCollection: userCollection,
		sessionStore:   sessionStore,
	}
}
