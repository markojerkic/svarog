package serverauth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type NatsAuthCalloutHandler struct {
	natsIssuerKeyPair nkeys.KeyPair
	natsAuthUser      string
	natsAuthPassword  string
	natsAddr          string
	jwtSecret         []byte
}

type NatsAuthClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

func NewNatsAuthCalloutHandler() *NatsAuthCalloutHandler {
	issuerSeed := os.Getenv("NATS_ISSUER_SEED")
	if issuerSeed == "" {
		panic("NATS_ISSUER_SEED is not set")
	}

	jwtSecret := os.Getenv("NATS_JWT_SECRET")
	if jwtSecret == "" {
		panic("NATS_JWT_SECRET is not set")
	}

	issuerKp, err := nkeys.FromSeed([]byte(issuerSeed))
	if err != nil {
		panic(err)
	}

	return &NatsAuthCalloutHandler{
		natsIssuerKeyPair: issuerKp,
		natsAuthUser:      os.Getenv("NATS_SYSTEM_USER"),
		natsAuthPassword:  os.Getenv("NATS_SYSTEM_PASSWORD"),
		natsAddr:          os.Getenv("NATS_ADDR"),
		jwtSecret:         []byte(jwtSecret),
	}
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
			return
		}

		log.Debug("JWT validated", "user_id", claims.UserID, "username", claims.Username)
	})

	return err
}

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
