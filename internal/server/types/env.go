package types

type ServerEnv struct {
	MongoUrl       string `env:"MONGO_URL"`
	GrpcServerPort int    `env:"GPRC_PORT"`
	ServerDnsName  string `env:"SERVER_DNS_NAME"`
	HttpServerPort int    `env:"HTTP_SERVER_PORT"`
	SessionSecret  string `env:"SESSION_SECRET"`

	NatsAddr           string `env:"NATS_ADDR"`
	NatsAccountSeed    string `env:"NATS_ACCOUNT_SEED"`
	NatsServerUserJWT  string `env:"NATS_SERVER_USER_JWT"`
	NatsServerUserSeed string `env:"NATS_SERVER_USER_SEED"`
}
