package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

const sessionDuration = 24 * time.Hour

type PostgresSessionStore struct {
	queries *sqlc.Queries
	codecs  []securecookie.Codec
}

var _ sessions.Store = (*PostgresSessionStore)(nil)

func NewPostgresSessionStore(pool *pgxpool.Pool, secretKey []byte) *PostgresSessionStore {
	return &PostgresSessionStore{queries: sqlc.New(pool), codecs: securecookie.CodecsFromPairs(secretKey)}
}

func (s *PostgresSessionStore) New(r *http.Request, name string) (*sessions.Session, error) {
	sess := sessions.NewSession(s, name)
	sess.Options = &sessions.Options{
		Path: "/", MaxAge: int(sessionDuration.Seconds()), HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	}
	return sess, nil
}

func (s *PostgresSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	sess, err := s.New(r, name)
	if err != nil {
		return nil, err
	}
	cookie, err := r.Cookie(name)
	if errors.Is(err, http.ErrNoCookie) {
		return sess, nil
	}
	if err != nil {
		return nil, err
	}
	var id string
	if err := securecookie.DecodeMulti(name, cookie.Value, &id, s.codecs...); err != nil {
		return nil, err
	}
	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	stored, err := s.queries.TouchSession(r.Context(), sessionID)
	if err != nil {
		return nil, err
	}
	sess.ID = stored.ID.String()
	sess.Values["user_id"] = stored.UserID.String()
	sess.IsNew = false
	return sess, nil
}

func (s *PostgresSessionStore) Save(r *http.Request, w http.ResponseWriter, sess *sessions.Session) error {
	if sess.Options == nil {
		return errors.New("Session options are missing")
	}
	if sess.Options.MaxAge <= 0 {
		if sess.ID != "" {
			id, err := uuid.Parse(sess.ID)
			if err != nil {
				return err
			}
			if err := s.queries.DeleteSession(r.Context(), id); err != nil {
				return err
			}
		}
		return s.setSessionCookie(w, sess)
	}
	userID, ok := sess.Values["user_id"].(string)
	if !ok {
		return errors.New("Session does not have a user ID")
	}
	id, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	if sess.ID == "" {
		if err := s.queries.DeleteExpiredSessions(r.Context()); err != nil {
			return err
		}
		stored, err := s.queries.CreateSession(r.Context(), sqlc.CreateSessionParams{
			ID: uuid.New(), UserID: id, ExpiresAt: time.Now().Add(sessionDuration),
		})
		if err != nil {
			return err
		}
		sess.ID = stored.ID.String()
	} else {
		sessionID, err := uuid.Parse(sess.ID)
		if err != nil {
			return err
		}
		if _, err := s.queries.UpdateSession(r.Context(), sqlc.UpdateSessionParams{ID: sessionID, UserID: id}); err != nil {
			return err
		}
	}
	sess.IsNew = false
	return s.setSessionCookie(w, sess)
}

func (s *PostgresSessionStore) setSessionCookie(w http.ResponseWriter, sess *sessions.Session) error {
	encoded, err := securecookie.EncodeMulti(sess.Name(), sess.ID, s.codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(sess.Name(), encoded, sess.Options))
	return nil
}
