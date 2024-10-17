package auth

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, username string, password string) (bool, error)
	Register(ctx context.Context, username string, password string) error
	GetCurrentUser(ctx echo.Context) (LoggedInUser, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
}

type MongoAuthService struct {
	mongoClient       *mongo.Client
	userCollection    *mongo.Collection
	sessionCollection *mongo.Collection
}

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
func (self *MongoAuthService) Register(ctx context.Context, username string, password string) error {
	// Check if user already exists
	existingUserResult := self.userCollection.FindOne(ctx, bson.M{
		"username": username,
	})
	if existingUserResult.Err() == nil {
		return fmt.Errorf("User %s already exists", username)
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	_, err = self.userCollection.InsertOne(ctx, User{
		Username: username,
		Password: hashedPassword,
		Role:     USER,
	})

	return nil
}

// GetCurrentUser implements AuthService.
func (m *MongoAuthService) GetCurrentUser(ctx echo.Context) (LoggedInUser, error) {
	panic("unimplemented")
}

// Login implements AuthService.
func (m *MongoAuthService) Login(ctx context.Context, username string, password string) (bool, error) {
	panic("unimplemented")
}

var _ AuthService = &MongoAuthService{}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func NewMongoAuthService(mongoClient *mongo.Client) *MongoAuthService {
	userCollection := mongoClient.Database("svarog").Collection("users")
	sessionCollection := mongoClient.Database("svarog").Collection("sessions")

	return &MongoAuthService{
		mongoClient:       mongoClient,
		userCollection:    userCollection,
		sessionCollection: sessionCollection,
	}
}
