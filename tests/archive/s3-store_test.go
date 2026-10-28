package archive_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/s3mock"
)

const s3PartSize = 16 * 1024 * 1024

func TestS3Store(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	const bucket = "archives"
	client := newS3MockClient(t, ctx, bucket)
	store, err := archive.NewS3Store(client, bucket)
	require.NoError(t, err, "create an S3 store backed by the test bucket")

	// Spans two parts with varying bytes, so reordered or duplicated parts change the checksum.
	large := make([]byte, s3PartSize+1024*1024+137)
	for i := range large {
		large[i] = byte(i*31 + i/255)
	}

	t.Run("round trip preserves exact content and metadata", func(t *testing.T) {
		for _, scenario := range []struct {
			name    string
			content []byte
		}{
			{name: "empty"},
			{name: "small", content: []byte("archive bytes\x00with binary content")},
			{name: "multipart", content: large},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				key := "round-trip/" + scenario.name + ".zip"
				object, err := store.Put(ctx, key, func(output io.Writer) error {
					_, err := io.Copy(output, bytes.NewReader(scenario.content))
					return err
				})
				require.NoError(t, err, "a successful Put makes all archive bytes available")
				require.Equal(t, int64(len(scenario.content)), object.SizeBytes, "stored size describes the exact uploaded bytes")
				require.Equal(t, sha256.Sum256(scenario.content), object.SHA256, "stored SHA-256 hashes the entire object rather than its multipart ETag")
				requireS3Content(t, ctx, store, key, scenario.content)

				if len(scenario.content) > s3PartSize {
					info, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
					require.NoError(t, err, "inspect the completed multipart object")
					require.Contains(t, aws.ToString(info.ETag), "-", "the large stream exercises S3 multipart completion")
				}
				for _, size := range []int64{object.SizeBytes - 1, object.SizeBytes + 1} {
					body, err := store.Open(ctx, key, size)
					if body != nil {
						_ = body.Close()
					}
					require.Error(t, err, "a size mismatch must never expose archive content")
					require.Nil(t, body, "rejected content must not return a download stream")
				}

				require.NoError(t, store.Delete(ctx, key), "remove completed archive content")
				require.NoError(t, store.Delete(ctx, key), "deleting missing content is idempotent")
				body, err := store.Open(ctx, key, object.SizeBytes)
				if body != nil {
					_ = body.Close()
				}
				require.Error(t, err, "deleted content is no longer downloadable")
			})
		}
	})

	t.Run("failed producers never publish incomplete objects", func(t *testing.T) {
		failure := errors.New("ZIP generation failed")
		for _, scenario := range []struct {
			name      string
			content   []byte
			failure   error
			multipart bool
		}{
			{name: "small", content: []byte("partial archive"), failure: failure},
			{name: "producer EOF", content: []byte("partial archive"), failure: io.EOF},
			{name: "uploaded part", content: large[:s3PartSize], failure: failure, multipart: true},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				key := "producer-failure/" + scenario.name + ".zip"
				object, err := store.Put(ctx, key, func(output io.Writer) error {
					if _, err := io.Copy(output, bytes.NewReader(scenario.content)); err != nil {
						return err
					}
					if scenario.multipart {
						waitForS3Part(t, ctx, client, bucket, key)
					}
					return scenario.failure
				})
				require.ErrorIs(t, err, scenario.failure, "producer failures remain recognizable to the archive service")
				require.Equal(t, archive.StoredObject{}, object, "a failed producer cannot return successful storage metadata")
				_, err = client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
				require.Error(t, err, "an incomplete archive is never published as an object")
				require.Empty(t, s3Uploads(t, ctx, client, bucket, key), "failed multipart production aborts already uploaded parts")
			})
		}
	})

	t.Run("cancellation unblocks a live multipart stream", func(t *testing.T) {
		key := "cancellation/archive.zip"
		uploadCtx, cancelUpload := context.WithTimeout(ctx, 15*time.Second)
		defer cancelUpload()
		_, err := store.Put(uploadCtx, key, func(output io.Writer) error {
			if _, err := io.Copy(output, bytes.NewReader(large[:s3PartSize])); err != nil {
				return err
			}
			waitForS3Part(t, ctx, client, bucket, key)
			cancelUpload()
			_, err := output.Write([]byte("must not block after cancellation"))
			return err
		})
		require.ErrorIs(t, err, context.Canceled, "cancellation interrupts both producer writes and upload reads")
		require.Empty(t, s3Uploads(t, ctx, client, bucket, key), "cancellation cleans multipart parts using an independent cleanup context")
		_, err = client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		require.Error(t, err, "a canceled stream never publishes its partial object")
	})

	t.Run("S3 failure unblocks the producer without regenerating content", func(t *testing.T) {
		missing, err := archive.NewS3Store(client, "missing-bucket")
		require.NoError(t, err, "construct a store without performing network IO")
		uploadCtx, cancelUpload := context.WithTimeout(ctx, 15*time.Second)
		defer cancelUpload()
		calls := 0
		_, err = missing.Put(uploadCtx, "client/archive.zip", func(output io.Writer) error {
			calls++
			_, err := io.Copy(output, bytes.NewReader(large))
			return err
		})
		var missingBucket smithy.APIError
		require.ErrorAs(t, err, &missingBucket, "the S3 failure is preserved rather than hanging until the deadline")
		require.Equal(t, "NoSuchBucket", missingBucket.ErrorCode(), "the underlying S3 error code remains available to callers")
		require.Equal(t, 1, calls, "an S3 error never reruns the archive generator")
	})

	t.Run("part retries preserve the checksum and a single producer invocation", func(t *testing.T) {
		var failed atomic.Bool
		retrying := faultyS3Client(t, client, func(w http.ResponseWriter, r *http.Request) bool {
			if r.Method != http.MethodPut || !r.URL.Query().Has("uploadId") || !failed.CompareAndSwap(false, true) {
				return false
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "<Error><Code>SlowDown</Code><Message>Retry this part</Message></Error>")
			return true
		})
		retryingStore, err := archive.NewS3Store(retrying, bucket)
		require.NoError(t, err, "create an S3 store behind the failure proxy")
		key := "retry/archive.zip"
		calls := 0
		object, err := retryingStore.Put(ctx, key, func(output io.Writer) error {
			calls++
			_, err := io.Copy(output, bytes.NewReader(large))
			return err
		})
		require.NoError(t, err, "a transient part failure is retried successfully")
		require.True(t, failed.Load(), "the test must exercise an actual failed multipart request")
		require.Equal(t, 1, calls, "part retries must replay buffered data, not rerun ZIP generation")
		require.Equal(t, int64(len(large)), object.SizeBytes, "replayed parts are counted only once")
		require.Equal(t, sha256.Sum256(large), object.SHA256, "part retries do not hash replayed bytes twice")
		requireS3Content(t, ctx, store, key, large)
	})

	t.Run("confirmation failure retains uploaded content until explicit cleanup", func(t *testing.T) {
		unconfirmed := faultyS3Client(t, client, func(w http.ResponseWriter, r *http.Request) bool {
			if r.Method != http.MethodHead {
				return false
			}
			w.WriteHeader(http.StatusForbidden)
			return true
		})
		unconfirmedStore, err := archive.NewS3Store(unconfirmed, bucket)
		require.NoError(t, err, "create a store whose upload succeeds but confirmation is denied")
		key := "unconfirmed/archive.zip"
		content := []byte("uploaded before confirmation failed")
		object, err := unconfirmedStore.Put(ctx, key, func(output io.Writer) error {
			_, err := output.Write(content)
			return err
		})
		require.Error(t, err, "a failed storage confirmation must not report a successful Put")
		require.Equal(t, archive.StoredObject{}, object, "unconfirmed content must not return successful storage metadata")
		requireS3Content(t, ctx, store, key, content)
		require.NoError(t, store.Delete(ctx, key), "only explicit caller-authorized cleanup removes unconfirmed content")
	})

	t.Run("delete removes only exact-key content and all staging artifacts", func(t *testing.T) {
		key := "cleanup/archive.zip"
		neighbor := key + ".other"
		content := []byte("completed archive")
		for _, name := range []string{key, neighbor} {
			_, err := store.Put(ctx, name, func(output io.Writer) error {
				_, err := output.Write(content)
				return err
			})
			require.NoError(t, err, "prepare completed content for cleanup isolation")
		}
		for range 3 {
			stageS3Part(t, ctx, client, bucket, key)
		}
		neighborUpload := stageS3Part(t, ctx, client, bucket, neighbor)
		require.NoError(t, store.Delete(ctx, key), "delete completed content and every unfinished upload for the exact key")
		require.NoError(t, store.Delete(ctx, key), "repeating cleanup is safe even with neighboring uploads")
		require.Empty(t, s3Uploads(t, ctx, client, bucket, key), "cleanup aborts every unfinished upload for the exact object key")
		remaining := s3Uploads(t, ctx, client, bucket, neighbor)
		require.Len(t, remaining, 1, "cleanup must not abort another key sharing the requested prefix")
		require.Equal(t, neighborUpload, aws.ToString(remaining[0].UploadId), "the neighboring upload remains untouched")
		requireS3Content(t, ctx, store, neighbor, content)
		_, err = client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		require.Error(t, err, "cleanup also removes the completed object")
	})

	t.Run("invalid keys cannot reach stored archives", func(t *testing.T) {
		key := "protected.zip"
		content := []byte("keep this archive")
		_, err := store.Put(ctx, key, func(output io.Writer) error {
			_, err := output.Write(content)
			return err
		})
		require.NoError(t, err, "prepare an archive that invalid calls must not alter")
		for _, unsafe := range []string{"", ".", "../protected.zip", "/protected.zip", "client/../protected.zip", "client\\protected.zip"} {
			_, err := store.Put(ctx, unsafe, func(io.Writer) error {
				t.Error("invalid keys must be rejected before generating archive content")
				return nil
			})
			require.ErrorIs(t, err, archive.ErrInvalidInput, "unsafe keys cannot be used to upload archive content")
			require.ErrorIs(t, store.Delete(ctx, unsafe), archive.ErrInvalidInput, "unsafe keys cannot delete another archive")
			_, err = store.Open(ctx, unsafe, int64(len(content)))
			require.ErrorIs(t, err, archive.ErrInvalidInput, "unsafe keys cannot expose another archive")
		}
		requireS3Content(t, ctx, store, key, content)
	})
}

func newS3MockClient(t *testing.T, ctx context.Context, bucket string) *s3.Client {
	t.Helper()
	container, err := s3mock.Run(ctx, "adobe/s3mock:5.2.3", s3mock.WithInitialBuckets(bucket))
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start Adobe S3Mock with an isolated archive bucket")
	endpoint, err := container.HTTPSEndpointURL(ctx)
	require.NoError(t, err, "resolve the S3Mock HTTPS endpoint on a dynamically mapped port")
	// S3Mock misreads SDK checksum headers over plain HTTP, so use its
	// self-signed HTTPS endpoint. Only this test client skips verification.
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	t.Cleanup(httpClient.CloseIdleConnections)
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint), Region: "us-east-1", UsePathStyle: true,
		Credentials:      credentials.NewStaticCredentialsProvider("test", "test", ""),
		RetryMaxAttempts: 2, HTTPClient: httpClient,
	})
}

// faultyS3Client proxies client's requests to S3Mock unless fault answers them.
func faultyS3Client(t *testing.T, client *s3.Client, fault func(http.ResponseWriter, *http.Request) bool) *s3.Client {
	t.Helper()
	upstream, err := url.Parse(aws.ToString(client.Options().BaseEndpoint))
	require.NoError(t, err, "parse the S3Mock endpoint for a fault-injecting proxy")
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = client.Options().HTTPClient.(*http.Client).Transport
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !fault(w, r) {
			proxy.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return s3.New(client.Options(), func(options *s3.Options) {
		options.BaseEndpoint = aws.String(server.URL)
	})
}

func requireS3Content(t *testing.T, ctx context.Context, store archive.Store, key string, expected []byte) {
	t.Helper()
	body, err := store.Open(ctx, key, int64(len(expected)))
	require.NoError(t, err, "completed content is immediately available through Store.Open")
	defer body.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, body)
	require.NoError(t, err, "the entire stored archive stream can be read")
	require.Equal(t, int64(len(expected)), size, "the download contains exactly the uploaded byte count")
	require.Equal(t, sha256.Sum256(expected), [32]byte(hash.Sum(nil)), "downloaded bytes match the original producer's bytes")
}

func s3Uploads(t *testing.T, ctx context.Context, client *s3.Client, bucket, key string) []types.MultipartUpload {
	t.Helper()
	var uploads []types.MultipartUpload
	pages := s3.NewListMultipartUploadsPaginator(client, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket), Prefix: aws.String(key),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		require.NoError(t, err, "list unfinished uploads to verify exact-key cleanup")
		for _, upload := range page.Uploads {
			if aws.ToString(upload.Key) == key {
				uploads = append(uploads, upload)
			}
		}
	}
	return uploads
}

func waitForS3Part(t *testing.T, ctx context.Context, client *s3.Client, bucket, key string) {
	t.Helper()
	require.Eventually(t, func() bool {
		uploads, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
			Bucket: aws.String(bucket), Prefix: aws.String(key),
		})
		if err != nil {
			return false
		}
		for _, upload := range uploads.Uploads {
			if aws.ToString(upload.Key) != key {
				continue
			}
			parts, err := client.ListParts(ctx, &s3.ListPartsInput{
				Bucket: aws.String(bucket), Key: aws.String(key), UploadId: upload.UploadId,
			})
			if err == nil && len(parts.Parts) > 0 {
				return true
			}
		}
		return false
	}, 10*time.Second, 10*time.Millisecond, "a multipart part must reach S3 before the producer fails or cancels")
}

func stageS3Part(t *testing.T, ctx context.Context, client *s3.Client, bucket, key string) string {
	t.Helper()
	upload, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
	})
	require.NoError(t, err, "prepare an interrupted multipart upload")
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(bucket), Key: aws.String(key), UploadId: upload.UploadId,
		PartNumber: aws.Int32(1), Body: bytes.NewReader([]byte("unfinished content")),
	})
	require.NoError(t, err, "persist staging bytes for an interrupted multipart upload")
	return aws.ToString(upload.UploadId)
}
