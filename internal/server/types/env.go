package types

type ServerEnv struct {
	PostgresURL    string `env:"POSTGRES_URL" envDefault:"postgres://svarog:svarog@localhost:5434/svarog?sslmode=disable"`
	ServerDnsName  string `env:"SERVER_DNS_NAME"`
	HttpServerPort int    `env:"HTTP_SERVER_PORT"`
	SessionSecret  string `env:"SESSION_SECRET"`
	ArchiveDir     string `env:"ARCHIVE_DIR"`

	ArchiveS3Bucket          string `env:"ARCHIVE_S3_BUCKET"`
	ArchiveS3Endpoint        string `env:"ARCHIVE_S3_ENDPOINT"`
	ArchiveS3Region          string `env:"ARCHIVE_S3_REGION" envDefault:"auto"`
	ArchiveS3AccessKeyID     string `env:"ARCHIVE_S3_ACCESS_KEY_ID"`
	ArchiveS3SecretAccessKey string `env:"ARCHIVE_S3_SECRET_ACCESS_KEY"`

	NatsPublicAddr     string `env:"NATS_PUBLIC_ADDR"`
	NatsAddr           string `env:"NATS_ADDR"`
	NatsAccountSeed    string `env:"NATS_ACCOUNT_SEED"`
	NatsServerUserJWT  string `env:"NATS_SERVER_USER_JWT"`
	NatsServerUserSeed string `env:"NATS_SERVER_USER_SEED"`
}
