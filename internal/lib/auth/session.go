package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoSessionStore struct {
	mongoClient       *mongo.Client
	sessionCollection *mongo.Collection
	userCollection    *mongo.Collection

	secretKey []byte
}

// Get implements sessions.Store.
func (self *MongoSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	rawSession, err := sessions.GetRegistry(r).Get(self, name)
	if err != nil {
		return nil, err
	}

	var session Session
	err = self.sessionCollection.FindOne(r.Context(), bson.M{
		"_id": rawSession.ID,
	}).Decode(&session)
	if err != nil {
		return nil, err
	}

	if session.UserID.IsZero() {
		return nil, errors.New("Session does not have a user")
	}

	if session.Modified.Add(24 * time.Hour).Before(time.Now()) {
		return nil, errors.New("Session has expired")
	}

	session.Modified = time.Now()
	_, err = self.sessionCollection.UpdateOne(r.Context(), bson.M{
		"_id": rawSession.ID,
	}, bson.M{
		"$set": bson.M{
			"modified": session.Modified,
		},
	})
	if err != nil {
		return nil, err
	}

	return rawSession, nil
}

// New implements sessions.Store.
func (self *MongoSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	newSession := sessions.NewSession(self, name)
	newSession.Options = &sessions.Options{
		Path: "/",
		// 30 minutes
		MaxAge:   30 * 60 * 1000,
		Domain:   r.URL.Host,
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
	}
	newSession.IsNew = true

	return newSession, nil
}

// Save implements sessions.Store.
func (self *MongoSessionStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	userId, ok := session.Values["user_id"].(string)
	if !ok || userId == "" {
		return errors.New("Session does not have a user")
	}
	userObjectId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return err
	}

	// Verify that the user exists
	userResult := self.userCollection.FindOne(r.Context(), bson.M{
		"_id": userObjectId,
	})
	if userResult.Err() != nil {
		return errors.New("User does not exist")
	}

	insertResult, err := self.sessionCollection.InsertOne(r.Context(), Session{
		UserID:   userObjectId,
		Modified: time.Now(),
	})
	if err != nil {
		return errors.Join(errors.New("Could not save session"), err)
	}
	session.ID = insertResult.InsertedID.(primitive.ObjectID).Hex()

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, securecookie.CodecsFromPairs(self.secretKey)...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))

	return nil
}

var _ sessions.Store = &MongoSessionStore{}

func NewMongoSessionStore(mongoClient *mongo.Client, secretKey []byte) *MongoSessionStore {
	return &MongoSessionStore{
		mongoClient:       mongoClient,
		sessionCollection: mongoClient.Database("svarog").Collection("sessions"),
		userCollection:    mongoClient.Database("svarog").Collection("users"),
		secretKey:         secretKey,
	}
}
