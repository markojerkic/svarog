package types

type ServerEnv struct {
	MongoUrl                 string   `env:"MONGO_URL"`
	GrpcServerPort           int      `env:"GPRC_PORT"`
	ServerDnsName            string   `env:"SERVER_DNS_NAME"`
	HttpServerPort           int      `env:"HTTP_SERVER_PORT"`
	HttpServerAllowedOrigins []string `env:"HTTP_SERVER_ALLOWED_ORIGINS"`

	NatsAddr string `env:"NATS_ADDR"`

	// Account seed for signing client user JWTs
	NatsAccountSeed string `env:"NATS_ACCOUNT_SEED"`

	// Server user credentials for server to connect to NATS
	// Generated once with: go run cmd/nats-setup/main.go
	NatsServerUserJWT  string `env:"NATS_SERVER_USER_JWT"`
	NatsServerUserSeed string `env:"NATS_SERVER_USER_SEED"`
}
