package app

import (
	"os"
	"sync"
	"time"

	envParser "github.com/caarlos0/env/v11"
	"github.com/charmbracelet/log"
	dotenv "github.com/joho/godotenv"
)

type Config struct {
	LogLevel                      string        `env:"LOG_LEVEL" envDefault:"info"`
	LogConnectionString           string        `env:"SVAROG_CONN_STRING"`
	LogQueueCapacity              string        `env:"SVAROG_LOG_QUEUE_CAPACITY"`
	PostgresURL                   string        `env:"POSTGRES_URL" envDefault:"postgres://svarog:svarog@localhost:5434/svarog?sslmode=disable"`
	ServerDnsName                 string        `env:"SERVER_DNS_NAME"`
	HttpServerPort                int           `env:"HTTP_SERVER_PORT"`
	SessionSecret                 string        `env:"SESSION_SECRET"`
	ArchiveDir                    string        `env:"ARCHIVE_DIR"`
	ArchiveSchedulerEnabled       bool          `env:"ARCHIVE_SCHEDULER_ENABLED" envDefault:"true"`
	IngestStreamMaxBytes          int64         `env:"INGEST_STREAM_MAX_BYTES" envDefault:"0"`
	IngestAckWait                 time.Duration `env:"INGEST_ACK_WAIT" envDefault:"60s"`
	IngestAllowRetentionMigration bool          `env:"INGEST_ALLOW_RETENTION_MIGRATION" envDefault:"false"`

	ArchiveS3Bucket          string `env:"ARCHIVE_S3_BUCKET"`
	ArchiveS3Endpoint        string `env:"ARCHIVE_S3_ENDPOINT"`
	ArchiveS3Region          string `env:"ARCHIVE_S3_REGION" envDefault:"auto"`
	ArchiveS3AccessKeyID     string `env:"ARCHIVE_S3_ACCESS_KEY_ID"`
	ArchiveS3SecretAccessKey string `env:"ARCHIVE_S3_SECRET_ACCESS_KEY"`

	NatsPublicAddr  string `env:"NATS_PUBLIC_ADDR"`
	NatsAccountSeed string `env:"NATS_ACCOUNT_SEED"`
}

var loadDotenv sync.Once

// LoadEnv reads the environment for a command's configuration.
func LoadEnv[T any]() T {
	loadDotenv.Do(func() {
		if os.Getenv("DOCKER") == "true" {
			return
		}
		// Local values override .env defaults, but never the process environment.
		if err := dotenv.Load(".env.local"); err != nil && !os.IsNotExist(err) {
			log.Fatal("Error loading .env.local file", "error", err)
		}
		if err := dotenv.Load(); err != nil {
			log.Fatal("Error loading .env file", "error", err)
		}
	})

	var cfg T
	if err := envParser.Parse(&cfg); err != nil {
		log.Fatal("Error parsing env", "error", err)
	}
	return cfg
}
