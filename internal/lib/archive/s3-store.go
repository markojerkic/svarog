package archive

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Store struct {
	client   *s3.Client
	uploader *transfermanager.Client
	bucket   string
}

var _ Store = (*S3Store)(nil)

// NewS3Store stores archives in an existing bucket. The client owns endpoint,
// credentials and retry configuration.
func NewS3Store(client *s3.Client, bucket string) (Store, error) {
	if client == nil || strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("%w: S3 client and bucket are required", ErrInvalidInput)
	}
	return &S3Store{
		client: client,
		bucket: bucket,
		uploader: transfermanager.New(client, func(options *transfermanager.Options) {
			// Memory stays bounded to a few part buffers; the 10,000-part limit
			// caps an archive of unknown length at about 156 GiB.
			options.PartSizeBytes = 16 * 1024 * 1024
			options.MultipartUploadThreshold = options.PartSizeBytes
			options.Concurrency = 2
			options.FailTimeout = 5 * time.Second
		}),
	}, nil
}

func (s *S3Store) Backend() string {
	return "s3"
}

func (s *S3Store) Put(ctx context.Context, key string, write func(io.Writer) error) (StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	if err := validateS3Key(key); err != nil {
		return StoredObject{}, err
	}
	reader, writer := io.Pipe()
	// Close only the reader on exit; closing the writer signals a complete upload.
	defer reader.Close()
	// Cancellation must unblock both a pending SDK read and a producer Write.
	stop := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
	})
	defer stop()

	var uploaded *transfermanager.UploadObjectOutput
	done := make(chan error, 1)
	go func() {
		var err error
		uploaded, err = s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Bucket: aws.String(s.bucket), Key: aws.String(key),
			Body: reader, ContentType: aws.String("application/zip"),
		})
		// Unblock the producer if the upload stops reading early.
		_ = reader.CloseWithError(err)
		done <- err
	}()

	hash := sha256.New()
	writeErr := write(io.MultiWriter(writer, hash))
	if writeErr != nil {
		// Wrapping keeps a producer's io.EOF from completing the upload.
		writeErr = fmt.Errorf("write archive content: %w", writeErr)
	}
	_ = writer.CloseWithError(writeErr)
	if err := errors.Join(writeErr, <-done, ctx.Err()); err != nil {
		return StoredObject{}, fmt.Errorf("upload archive content: %w", err)
	}

	size := aws.ToInt64(uploaded.ContentLength)
	info, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return StoredObject{}, fmt.Errorf("confirm archive content: %w", err)
	}
	if info.ContentLength == nil || *info.ContentLength != size {
		return StoredObject{}, errors.New("stored archive size does not match uploaded content")
	}
	return StoredObject{SizeBytes: size, SHA256: [32]byte(hash.Sum(nil))}, nil
}

func (s *S3Store) Open(ctx context.Context, key string, sizeBytes int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateS3Key(key); err != nil {
		return nil, err
	}
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("open archive content: %w", err)
	}
	if object.ContentLength == nil || *object.ContentLength != sizeBytes {
		_ = object.Body.Close()
		return nil, errors.New("stored archive size does not match expected size")
	}
	return object.Body, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateS3Key(key); err != nil {
		return err
	}
	var cleanupErr error
	pages := s3.NewListMultipartUploadsPaginator(s.client, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(s.bucket), Prefix: aws.String(key),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("list unfinished archive uploads: %w", err))
			break
		}
		for _, upload := range page.Uploads {
			// The prefix also matches longer keys.
			if aws.ToString(upload.Key) != key {
				continue
			}
			_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket: aws.String(s.bucket), Key: aws.String(key), UploadId: upload.UploadId,
			})
			var missing *types.NoSuchUpload
			if err != nil && !errors.As(err, &missing) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("abort unfinished archive upload: %w", err))
			}
		}
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	}); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete archive content: %w", err))
	}
	return cleanupErr
}

func validateS3Key(key string) error {
	if key == "." || !fs.ValidPath(key) || strings.Contains(key, "\\") {
		return fmt.Errorf("%w: archive object key must be slash-separated and relative", ErrInvalidInput)
	}
	return nil
}
