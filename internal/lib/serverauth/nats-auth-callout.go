package serverauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type NatsAuthConfig struct {
	IssuerSeed     string
	JwtSecret      string
	SystemUser     string
	SystemPassword string
	NatsAddr       string
}

type NatsAuthCalloutHandler struct {
	natsIssuerKeyPair nkeys.KeyPair
	natsAuthUser      string
	natsAuthPassword  string
	natsAddr          string
	jwtSecret         []byte
}

type NatsAuthClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username,omitempty"`
	Topic    string `json:"topic,omitempty"`
}

func NewNatsAuthCalloutHandler(config NatsAuthConfig) (*NatsAuthCalloutHandler, error) {
	if config.IssuerSeed == "" {
		return nil, errors.New("IssuerSeed is required")
	}
	if config.JwtSecret == "" {
		return nil, errors.New("JwtSecret is required")
	}

	issuerKp, err := nkeys.FromSeed([]byte(config.IssuerSeed))
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer seed: %w", err)
	}

	return &NatsAuthCalloutHandler{
		natsIssuerKeyPair: issuerKp,
		natsAuthUser:      config.SystemUser,
		natsAuthPassword:  config.SystemPassword,
		natsAddr:          config.NatsAddr,
		jwtSecret:         []byte(config.JwtSecret),
	}, nil
}

func (n *NatsAuthCalloutHandler) Run() error {
	nc, err := nats.Connect(n.natsAddr,
		nats.UserInfo(n.natsAuthUser, n.natsAuthPassword),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return errors.Join(errors.New("nats connect failed"), err)
	}

	_, err = nc.Subscribe("$SYS.REQ.USER.AUTH", func(msg *nats.Msg) {
		reqClaim, err := natsjwt.DecodeAuthorizationRequestClaims(string(msg.Data))
		if err != nil {
			log.Error("nats auth callout", "err", err)
			return
		}
		token := reqClaim.ConnectOptions.Token
		log.Debug("Auth request", "user_nkey", reqClaim.UserNkey, "token_present", token != "", "server", reqClaim.Server.ID)

		claims, err := n.ValidateJWT(token)
		if err != nil {
			log.Error("JWT validation failed", "err", err)
			n.respondWithError(msg, reqClaim, "invalid token")
			return
		}

		log.Debug("JWT validated", "username", claims.Username, "topic", claims.Topic)

		if err := n.respondWithSuccess(msg, reqClaim, claims); err != nil {
			log.Error("Failed to respond with success", "err", err)
		}
	})

	return err
}

// GenerateToken creates a signed JWT token that grants access to a specific topic.
func (n *NatsAuthCalloutHandler) GenerateToken(username, topic string) (string, error) {
	if topic == "" {
		return "", errors.New("topic is required")
	}

	claims := NatsAuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		Username: username,
		Topic:    topic,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(n.jwtSecret)
}

func (n *NatsAuthCalloutHandler) respondWithSuccess(msg *nats.Msg, reqClaim *natsjwt.AuthorizationRequestClaims, claims *NatsAuthClaims) error {
	userClaim := natsjwt.NewUserClaims(reqClaim.UserNkey)
	userClaim.Audience = "APP"
	userClaim.Name = claims.Username

	// Grant publish permission only to the topic from JWT
	userClaim.Permissions.Pub.Allow.Add(claims.Topic)

	// Sign the response
	response, err := userClaim.Encode(n.natsIssuerKeyPair)
	if err != nil {
		return fmt.Errorf("failed to encode user claims: %w", err)
	}

	// Create authorization response
	authResponse := natsjwt.NewAuthorizationResponseClaims(reqClaim.UserNkey)
	authResponse.Audience = reqClaim.Server.ID
	authResponse.Jwt = response

	responseJwt, err := authResponse.Encode(n.natsIssuerKeyPair)
	if err != nil {
		return fmt.Errorf("failed to encode auth response: %w", err)
	}

	if err := msg.Respond([]byte(responseJwt)); err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	log.Debug("Auth success", "username", claims.Username, "topic", claims.Topic)
	return nil
}

func (n *NatsAuthCalloutHandler) respondWithError(msg *nats.Msg, reqClaim *natsjwt.AuthorizationRequestClaims, errorMsg string) {
	authResponse := natsjwt.NewAuthorizationResponseClaims(reqClaim.UserNkey)
	authResponse.Audience = reqClaim.Server.ID
	authResponse.Error = errorMsg

	responseJwt, err := authResponse.Encode(n.natsIssuerKeyPair)
	if err != nil {
		log.Error("Failed to encode error response", "err", err)
		return
	}

	if err := msg.Respond([]byte(responseJwt)); err != nil {
		log.Error("Failed to send error response", "err", err)
	}
}

// ValidateJWT validates the JWT token and returns the claims if valid.
func (n *NatsAuthCalloutHandler) ValidateJWT(tokenString string) (*NatsAuthClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &NatsAuthClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return n.jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*NatsAuthClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
