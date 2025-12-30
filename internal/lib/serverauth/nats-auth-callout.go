package serverauth

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type NatsAuthConfig struct {
	IssuerSeed string
}

type NatsAuthCalloutHandler struct {
	natsIssuerKeyPair nkeys.KeyPair
	tokenService      *TokenService
	conn              *nats.Conn
}

type NatsAuthClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username,omitempty"`
	Topic    string `json:"topic,omitempty"`
}

func NewNatsAuthCalloutHandler(config NatsAuthConfig, conn *nats.Conn, tokenService *TokenService) (*NatsAuthCalloutHandler, error) {
	if config.IssuerSeed == "" {
		return nil, errors.New("IssuerSeed is required")
	}
	if conn == nil {
		return nil, errors.New("NATS connection is required")
	}
	if tokenService == nil {
		return nil, errors.New("TokenService is required")
	}

	issuerKp, err := nkeys.FromSeed([]byte(config.IssuerSeed))
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer seed: %w", err)
	}

	return &NatsAuthCalloutHandler{
		natsIssuerKeyPair: issuerKp,
		tokenService:      tokenService,
		conn:              conn,
	}, nil
}

func (n *NatsAuthCalloutHandler) Run() error {
	_, err := n.conn.Subscribe("$SYS.REQ.USER.AUTH", func(msg *nats.Msg) {
		reqClaim, err := natsjwt.DecodeAuthorizationRequestClaims(string(msg.Data))
		if err != nil {
			log.Error("nats auth callout", "err", err)
			return
		}
		token := reqClaim.ConnectOptions.Token

		claims, err := n.tokenService.ValidateJWT(token)
		if err != nil {
			log.Error("JWT validation failed", "err", err)
			n.respondWithError(msg, reqClaim, "invalid token")
			return
		}

		if err := n.respondWithSuccess(msg, reqClaim, claims); err != nil {
			log.Error("Failed to respond with success", "err", err)
		}
	})

	return err
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
