package types

type ServerEnv struct {
	MongoUrl                 string   `env:"MONGO_URL"`
	GrpcServerPort           int      `env:"GPRC_PORT"`
	ServerDnsName            string   `env:"SERVER_DNS_NAME"`
	HttpServerPort           int      `env:"HTTP_SERVER_PORT"`
	HttpServerAllowedOrigins []string `env:"HTTP_SERVER_ALLOWED_ORIGINS"`

	NatsAddr           string `env:"NATS_ADDR"`
	NatsIssuerSeed     string `env:"NATS_ISSUER_SEED"`
	NatsJwtSecret      string `env:"NATS_JWT_SECRET"`
	NatsSystemUser     string `env:"NATS_SYSTEM_USER"`
	NatsSystemPassword string `env:"NATS_SYSTEM_PASSWORD"`
}
