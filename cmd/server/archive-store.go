package main

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/server/types"
)

func newArchiveStore(env types.ServerEnv) (archive.Store, error) {
	if env.ArchiveS3Bucket == "" && env.ArchiveS3Endpoint == "" &&
		env.ArchiveS3AccessKeyID == "" && env.ArchiveS3SecretAccessKey == "" {
		return archive.NewDiskStore(env.ArchiveDir), nil
	}
	required := []struct {
		name  string
		value string
	}{
		{"ARCHIVE_S3_BUCKET", env.ArchiveS3Bucket},
		{"ARCHIVE_S3_ENDPOINT", env.ArchiveS3Endpoint},
		{"ARCHIVE_S3_REGION", env.ArchiveS3Region},
		{"ARCHIVE_S3_ACCESS_KEY_ID", env.ArchiveS3AccessKeyID},
		{"ARCHIVE_S3_SECRET_ACCESS_KEY", env.ArchiveS3SecretAccessKey},
	}
	for _, setting := range required {
		if strings.TrimSpace(setting.value) == "" {
			return nil, fmt.Errorf("%s is required for s3 archive storage", setting.name)
		}
	}
	client := s3.New(s3.Options{
		Region:       env.ArchiveS3Region,
		BaseEndpoint: aws.String(env.ArchiveS3Endpoint),
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			env.ArchiveS3AccessKeyID, env.ArchiveS3SecretAccessKey, "",
		)),
		UsePathStyle: true,
	})
	return archive.NewS3Store(client, env.ArchiveS3Bucket)
}
