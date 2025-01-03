package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoSessionStore struct {
	sessionCollection *mongo.Collection
	userCollection    *mongo.Collection

	secretKey []byte
}

// Get implements sessions.Store.
func (self *MongoSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(self, name)

	// Get the cookie
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}

	// Decode the session ID from the cookie
	var sessionID string
	err = securecookie.DecodeMulti(name, cookie.Value, &sessionID, securecookie.CodecsFromPairs(self.secretKey)...)
	if err != nil {
		return nil, err
	}

	// Convert session ID to ObjectID
	sessionObjID, err := primitive.ObjectIDFromHex(sessionID)
	if err != nil {
		return nil, err
	}

	// Find the session in database
	var dbSession Session
	err = self.sessionCollection.FindOne(r.Context(), bson.M{
		"_id": sessionObjID,
	}).Decode(&dbSession)
	if err != nil {
		return nil, err
	}

	if dbSession.UserID.IsZero() {
		return nil, errors.New("Session does not have a user")
	}

	if dbSession.Modified.Add(24 * time.Hour).Before(time.Now()) {
		// Delete expired session
		_, _ = self.sessionCollection.DeleteOne(r.Context(), bson.M{"_id": sessionObjID})
		return nil, errors.New("Session has expired")
	}

	// Update session modified time
	_, err = self.sessionCollection.UpdateOne(
		r.Context(),
		bson.M{"_id": sessionObjID},
		bson.M{"$set": bson.M{"modified": time.Now()}},
	)
	if err != nil {
		return nil, err
	}

	session.ID = sessionID
	session.Values["user_id"] = dbSession.UserID.Hex()
	session.IsNew = false

	return session, nil
}

// New implements sessions.Store.
func (self *MongoSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	newSession := sessions.NewSession(self, name)
	newSession.Options = &sessions.Options{
		Path: "/",
		// 30 minutes
		MaxAge:   30 * 60,
		Domain:   r.URL.Host,
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	newSession.IsNew = true

	return newSession, nil
}

// Save implements sessions.Store.
func (self *MongoSessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	userId, ok := session.Values["user_id"].(string)
	if !ok || userId == "" {
		return errors.New("session does not have a user ID")
	}

	userObjectId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Verify user exists
	if err := self.verifyUser(r.Context(), userObjectId); err != nil {
		return err
	}

	sessionData := Session{
		UserID:   userObjectId,
		Modified: time.Now(),
	}

	ctx := r.Context()
	var result *mongo.UpdateResult
	if session.ID != "" {
		// Update existing session
		result, err = self.sessionCollection.UpdateOne(ctx,
			bson.M{"_id": session.ID},
			bson.M{"$set": sessionData})
		slog.Debug("Update session", slog.Any("result", result))
	} else {
		// Create new session
		inserted, err := self.sessionCollection.InsertOne(ctx, sessionData)
		if err == nil {
			session.ID = inserted.InsertedID.(primitive.ObjectID).Hex()
		}
	}
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return self.setSessionCookie(w, session)
}

func (self *MongoSessionStore) verifyUser(ctx context.Context, userId primitive.ObjectID) error {
	var user User
	err := self.userCollection.FindOne(ctx, bson.M{"_id": userId}).Decode(&user)
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	return nil
}

func (self *MongoSessionStore) setSessionCookie(w http.ResponseWriter, session *sessions.Session) error {
	codecs := securecookie.CodecsFromPairs(self.secretKey)
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, codecs...)
	if err != nil {
		return err
	}

	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))

	return nil
}

var _ sessions.Store = &MongoSessionStore{}

const SESSION_DURATION = int32(24 * 60 * 60)

func CreateSessionCollection(db *mongo.Database) (*mongo.Collection, error) {
	err := db.CreateCollection(context.Background(), "sessions")
	if err != nil {
		return nil, errors.Join(errors.New("Error creating sessions collection"), err)
	}

	collection := db.Collection("sessions")

	// Create index on modified field
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "modified", Value: int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(int32(time.Now().Add(time.Hour * 24).Unix())), // Will be removed after 24 Hours.
	}

	_, err = collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		return nil, errors.Join(errors.New("Error creating index on sessions collection"), err)
	}

	return collection, nil

}

func NewMongoSessionStore(sessionCollection *mongo.Collection, userCollection *mongo.Collection, secretKey []byte) *MongoSessionStore {
	return &MongoSessionStore{
		sessionCollection: sessionCollection,
		userCollection:    userCollection,
		secretKey:         secretKey,
	}
}
