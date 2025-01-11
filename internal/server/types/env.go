package types

type ServerEnv struct {
	MongoUrl                 string   `env:"MONGO_URL"`
	GrpcServerPort           int      `env:"GPRC_PORT"`
	HttpServerPort           int      `env:"HTTP_SERVER_PORT"`
	HttpServerAllowedOrigins []string `env:"HTTP_SERVER_ALLOWED_ORIGINS"`
}
