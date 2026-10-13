package types

type ServerEnv struct {
	PostgresURL    string `env:"POSTGRES_URL" envDefault:"postgres://svarog:svarog@localhost:5434/svarog?sslmode=disable"`
	MongoUrl       string `env:"MONGO_URL"`
	ServerDnsName  string `env:"SERVER_DNS_NAME"`
	HttpServerPort int    `env:"HTTP_SERVER_PORT"`
	SessionSecret  string `env:"SESSION_SECRET"`

	NatsPublicAddr     string `env:"NATS_PUBLIC_ADDR"`
	NatsAddr           string `env:"NATS_ADDR"`
	NatsAccountSeed    string `env:"NATS_ACCOUNT_SEED"`
	NatsServerUserJWT  string `env:"NATS_SERVER_USER_JWT"`
	NatsServerUserSeed string `env:"NATS_SERVER_USER_SEED"`
}
