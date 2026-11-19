package embedded

import (
	"errors"
	"fmt"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

type brokerAuth struct {
	operator *jwt.OperatorClaims
	resolver *server.MemAccResolver
	userJWT  string
	userSeed string
}

func newBrokerAuth(accountSeed string) (brokerAuth, error) {
	appKey, err := nkeys.FromSeed([]byte(accountSeed))
	if err != nil {
		return brokerAuth{}, fmt.Errorf("NATS_ACCOUNT_SEED must be a valid account seed: %w", err)
	}
	defer appKey.Wipe()
	appPub, err := appKey.PublicKey()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("derive APP account public key: %w", err)
	}
	if !nkeys.IsValidPublicAccountKey(appPub) {
		return brokerAuth{}, errors.New("NATS_ACCOUNT_SEED must be an account seed")
	}

	operatorKey, err := nkeys.CreateOperator()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("create NATS operator key: %w", err)
	}
	defer operatorKey.Wipe()
	operatorPub, err := operatorKey.PublicKey()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("derive NATS operator public key: %w", err)
	}

	systemKey, err := nkeys.CreateAccount()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("create NATS system account key: %w", err)
	}
	defer systemKey.Wipe()
	systemPub, err := systemKey.PublicKey()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("derive NATS system account public key: %w", err)
	}

	operator := jwt.NewOperatorClaims(operatorPub)
	operator.Name = "Svarog-Operator"
	operator.SystemAccount = systemPub
	if _, err := operator.Encode(operatorKey); err != nil {
		return brokerAuth{}, fmt.Errorf("sign NATS operator claims: %w", err)
	}
	system := jwt.NewAccountClaims(systemPub)
	system.Name = "$SYS"
	app := jwt.NewAccountClaims(appPub)
	app.Name = "APP"
	app.Limits.DiskStorage = jwt.NoLimit
	app.Limits.MemoryStorage = jwt.NoLimit

	resolver := &server.MemAccResolver{}
	for _, account := range []*jwt.AccountClaims{system, app} {
		token, err := account.Encode(operatorKey)
		if err != nil {
			return brokerAuth{}, fmt.Errorf("sign %s account claims: %w", account.Name, err)
		}
		if err := resolver.Store(account.Subject, token); err != nil {
			return brokerAuth{}, fmt.Errorf("register %s account: %w", account.Name, err)
		}
	}

	userKey, err := nkeys.CreateUser()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("create embedded NATS user key: %w", err)
	}
	defer userKey.Wipe()
	userPub, err := userKey.PublicKey()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("derive embedded NATS user public key: %w", err)
	}
	userSeed, err := userKey.Seed()
	if err != nil {
		return brokerAuth{}, fmt.Errorf("read embedded NATS user seed: %w", err)
	}
	user := jwt.NewUserClaims(userPub)
	user.Name = "svarog-server"
	user.IssuerAccount = appPub
	user.Permissions.Pub.Allow.Add(">")
	user.Permissions.Sub.Allow.Add(">")
	userJWT, err := user.Encode(appKey)
	if err != nil {
		return brokerAuth{}, fmt.Errorf("sign embedded NATS user claims: %w", err)
	}
	return brokerAuth{operator: operator, resolver: resolver, userJWT: userJWT, userSeed: string(userSeed)}, nil
}
