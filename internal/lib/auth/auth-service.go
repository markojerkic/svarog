package auth

import (
	"context"
	"errors"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/sethvargo/go-password/password"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
)

type AuthService interface {
	Login(ctx echo.Context, username string, password string) error
	LoginWithToken(ctx echo.Context, token string) error
	Register(ctx echo.Context, form types.RegisterForm) (string, error)
	Logout(ctx echo.Context) error
	ResetPassword(ctx context.Context, userId string, form types.ResetPasswordForm) error
	DeleteUser(ctx echo.Context, id string) error
	GetCurrentUser(ctx echo.Context) (LoggedInUser, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserPage(ctx context.Context, query types.GetUserPageInput) ([]User, error)
}

type MongoAuthService struct {
	userCollection    *mongo.Collection
	sessionCollection *mongo.Collection
	mongoClient       *mongo.Client
	sessionStore      sessions.Store
}

const SVAROG_SESSION = "svarog_session"
const (
	ErrUserNotFound     = "User not found"
	UserAlreadyExists   = "User already exists"
	LoginTokenNotValid  = "Login token not valid"
	PasswordsDoNotMatch = "Passwords do not match"
) // Error codes

// ResetPassword implements AuthService.
func (self *MongoAuthService) ResetPassword(ctx context.Context, userId string, form types.ResetPasswordForm) error {

	if form.Password != form.RepeatedPassword {
		return errors.New(PasswordsDoNotMatch)
	}

	userID, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return err
	}

	_, err = util.StartTransaction(ctx, func(c mongo.SessionContext) (interface{}, error) {
		var user User
		err = self.userCollection.FindOne(ctx, bson.M{
			"_id": userID,
		}).Decode(&user)

		if err != nil {
			return struct{}{}, errors.New(ErrUserNotFound)
		}

		hashedPassword, err := hashPassword(form.Password)
		if err != nil {
			return struct{}{}, err
		}
		updateResult, err := self.userCollection.UpdateByID(ctx, user.ID, bson.M{
			"$set": bson.M{
				"password":             hashedPassword,
				"needs_password_reset": false,
			},
		})

		return updateResult, err

	}, self.mongoClient)
	return err
}

// GetUserByID implements AuthService.
func (self *MongoAuthService) GetUserByID(ctx context.Context, id string) (User, error) {
	var user User
	userId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return user, err
	}

	err = self.userCollection.FindOne(ctx, bson.M{
		"_id": userId,
	}).Decode(&user)

	return user, err
}

// Logout implements AuthService.
func (m *MongoAuthService) Logout(ctx echo.Context) error {
	session, err := session.Get(SVAROG_SESSION, ctx)
	if err != nil {
		return errors.New("Error getting session")
	}
	session.Options.MaxAge = -1
	return session.Save(ctx.Request(), ctx.Response())

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
func (m *MongoAuthService) Register(ctx echo.Context, form types.RegisterForm) (string, error) {
	// Check if user already exists
	existingUserResult := m.userCollection.FindOne(ctx.Request().Context(), bson.M{
		"username": form.Username,
	})
	if existingUserResult.Err() == nil {
		return "", errors.New(UserAlreadyExists)
	}

	hashedPassword, err := hashPassword(generateRandomPassword())
	if err != nil {
		return "", err
	}

	loginToken := generateLoginToken()
	user, err := m.userCollection.InsertOne(ctx.Request().Context(), User{
		Username:           form.Username,
		FirstName:          form.FirstName,
		LastName:           form.LastName,
		Password:           hashedPassword,
		Role:               USER,
		LoginTokens:        []string{loginToken},
		NeedsPasswordReset: true,
	})

	_, ok := user.InsertedID.(primitive.ObjectID)

	if !ok {
		return "", errors.New("Failed to get user ID")
	}

	return loginToken, nil
}

// GetCurrentUser implements AuthService.
func (m *MongoAuthService) GetCurrentUser(ctx echo.Context) (LoggedInUser, error) {
	session, err := session.Get(SVAROG_SESSION, ctx)
	if err != nil {
		return LoggedInUser{}, errors.Join(errors.New("Couldn't get session from context"), err)
	}

	userId, ok := session.Values["user_id"].(string)

	if !ok {
		slog.Error("User ID is not a string")
		return LoggedInUser{}, errors.New(ErrUserNotFound)
	}

	user, err := m.GetUserByID(ctx.Request().Context(), userId)

	if err != nil {
		slog.Error("Failed to get user by ID", "userId", userId, "error", err)
		return LoggedInUser{}, errors.Join(errors.New(ErrUserNotFound), err)
	}

	return LoggedInUser{
		ID:                 user.ID.Hex(),
		Username:           user.Username,
		Role:               user.Role,
		NeedsPasswordReset: user.NeedsPasswordReset,
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

func (m *MongoAuthService) LoginWithToken(ctx echo.Context, token string) error {
	_, err := util.StartTransaction(ctx.Request().Context(), func(sc mongo.SessionContext) (interface{}, error) {
		var user User
		err := m.userCollection.FindOne(ctx.Request().Context(), bson.M{
			"login_tokens": token,
		}).Decode(&user)
		if err != nil {
			return struct{}{}, errors.New(LoginTokenNotValid)
		}

		// remove token from db
		_, err = m.userCollection.UpdateByID(sc, user.ID, bson.M{
			"$pull": bson.M{
				"login_tokens": token,
			},
		})

		if err != nil {
			return struct{}{}, errors.Join(errors.New("Failed to delete login token"), err)
		}

		return struct{}{}, m.createSession(ctx, user.ID.Hex())
	}, m.mongoClient)
	return err

}

func (m *MongoAuthService) DeleteUser(ctx echo.Context, id string) error {
	_, err := util.StartTransaction(ctx.Request().Context(), func(c mongo.SessionContext) (interface{}, error) {
		user, err := m.GetUserByID(c, id)
		if err != nil {
			return struct{}{}, err
		}
		// Delete user
		_, err = m.userCollection.DeleteOne(c, bson.M{
			"_id": user.ID,
		})
		if err != nil {
			return struct{}{}, err
		}
		// Delete sessions
		_, err = m.sessionCollection.DeleteMany(c, bson.M{
			"user_id": user.ID,
		})
		if err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil

	}, m.mongoClient)

	return err
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

// GetUserPage implements AuthService.
func (self *MongoAuthService) GetUserPage(ctx context.Context, query types.GetUserPageInput) ([]User, error) {
	var users []User

	limit := query.Size
	skip := query.Page * query.Size

	cursor, err := self.userCollection.Find(ctx, bson.M{
		"username": bson.M{"$regex": query.Username},
	}, &options.FindOptions{
		Limit: &limit,
		Skip:  &skip,
		Projection: bson.M{
			"password": 0,
		},
	})
	if err != nil {
		return users, err
	}

	err = cursor.All(ctx, &users)
	if err != nil {
		return users, err
	}

	return users, nil
}

func (m *MongoAuthService) CreateInitialAdminUser(ctx context.Context) error {
	// Check if admin user already exists
	existingUserResult := m.userCollection.FindOne(ctx, bson.M{
		"username": "admin",
	})
	if existingUserResult.Err() == nil {
		slog.Warn("Admin user already exists, not creating")
		return nil
	}

	hashedPassword, err := hashPassword("ADMINADMIN")
	if err != nil {
		return err
	}

	_, err = m.userCollection.InsertOne(ctx, User{
		Username:           "admin",
		FirstName:          "Admin",
		LastName:           "Admin",
		Password:           hashedPassword,
		Role:               ADMIN,
		NeedsPasswordReset: true,
		LoginTokens:        []string{generateLoginToken()},
	})
	return err
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func generateRandomPassword() string {
	res, err := password.Generate(64, 10, 10, false, false)
	if err != nil {
		slog.Error("Failed to generate password", "error", err)
		panic(err)
	}

	return res
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateLoginToken() string {
	return primitive.NewObjectID().Hex()
}

var _ AuthService = &MongoAuthService{}

func NewMongoAuthService(userCollection *mongo.Collection, sessionCollection *mongo.Collection, client *mongo.Client, sessionStore sessions.Store) *MongoAuthService {
	return &MongoAuthService{
		userCollection:    userCollection,
		sessionCollection: sessionCollection,
		mongoClient:       client,
		sessionStore:      sessionStore,
	}
}
