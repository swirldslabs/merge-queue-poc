package storage

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"time"
)

type s3Handler struct {
	*handler
	client       s3Client
	bucketConfig config.BucketConfig
	retryConfig  config.RetryConfig
	bucketExists map[string]bool
}

// s3Client is an interface that defines the methods for interacting with S3-compatible storage.
// It is used to abstract the MinIO client to expose limited functionalities, which also allows for mocking in tests.
type s3Client interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)

	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error

	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)

	FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

// minioClientWrapper is a wrapper around the MinIO client to implement the s3Client interface.
type minioClientWrapper struct {
	client *minio.Client
}

func (m *minioClientWrapper) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return m.client.BucketExists(ctx, bucketName)
}

func (m *minioClientWrapper) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return m.client.MakeBucket(ctx, bucketName, opts)
}

func (m *minioClientWrapper) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return m.client.StatObject(ctx, bucketName, objectName, opts)
}

func (m *minioClientWrapper) FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return m.client.FPutObject(ctx, bucketName, objectName, filePath, opts)
}

// ensureBucketExists checks if the bucket exists in S3. If it doesn't exist, it creates the bucket.
func (s *s3Handler) ensureBucketExists(ctx context.Context) error {
	if _, exists := s.bucketExists[s.bucketConfig.Bucket]; exists {
		logx.As().Trace().
			Str("storage_type", s.Type()).
			Str("bucket", s.bucketConfig.Bucket).
			Msg("Bucket existence confirmed from cache")
		return nil
	}

	logx.As().Trace().
		Str("storage_type", s.Type()).
		Str("bucket", s.bucketConfig.Bucket).
		Msg("Checking if bucket exists")

	exists, err := s.client.BucketExists(ctx, s.bucketConfig.Bucket)
	if err != nil {
		return err
	}

	if !exists {
		logx.As().Trace().
			Str("storage_type", s.Type()).
			Str("bucket", s.bucketConfig.Bucket).
			Msg("Bucket does not exist, creating it")
		if err := s.client.MakeBucket(ctx, s.bucketConfig.Bucket, minio.MakeBucketOptions{Region: s.bucketConfig.Region}); err != nil {
			logx.As().Error().
				Str("storage_type", s.Type()).
				Str("bucket", s.bucketConfig.Bucket).
				Err(err).
				Msg("Failed to create bucket")
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		logx.As().Trace().
			Str("storage_type", s.Type()).
			Str("bucket", s.bucketConfig.Bucket).
			Msg("Bucket created successfully")
	}

	s.bucketExists[s.bucketConfig.Bucket] = true
	return nil
}

// syncWithBucket uploads a file to the S3 bucket. It skips the upload if the file already exists with the same checksum.
func (s *s3Handler) syncWithBucket(ctx context.Context, src, objectName string) (*core.UploadInfo, error) {
	logx.As().Info().
		Str("id", s.Info()).
		Str("src", src).
		Str("object", objectName).
		Str("bucket", s.bucketConfig.Bucket).
		Msg("Attempting to sync file with the bucket")

	localChecksum, err := fsx.FileMD5(src)
	if err != nil {
		logx.As().Error().
			Str("id", s.Info()).
			Str("src", src).
			Err(err).
			Msg("Failed to calculate local file checksum")
		return nil, fmt.Errorf("failed to calculate local checksum: %w", err)
	}

	attr, err := s.client.StatObject(ctx, s.bucketConfig.Bucket, objectName, minio.StatObjectOptions{})
	if err == nil && localChecksum == attr.ETag {
		logx.As().Info().
			Str("id", s.Info()).
			Str("src", src).
			Str("object", objectName).
			Str("md5", attr.ETag).
			Str("bucket", s.bucketConfig.Bucket).
			Time("last_modified", attr.LastModified).
			Msg("File already exists in bucket, skipping upload")
		return &core.UploadInfo{
			Src:          src,
			Dest:         attr.Key,
			ChecksumType: "md5",
			Checksum:     attr.ETag,
			Size:         attr.Size,
			LastModified: attr.LastModified,
		}, nil
	}

	logx.As().Debug().
		Str("id", s.Info()).
		Str("src", src).
		Str("object", objectName).
		Str("local_checksum", localChecksum).
		Str("bucket", s.bucketConfig.Bucket).
		Msg("Uploading file to bucket")

	info, err := s.client.FPutObject(ctx, s.bucketConfig.Bucket, objectName, src, minio.PutObjectOptions{
		SendContentMd5:        true,
		ConcurrentStreamParts: false,
	})
	if err != nil {
		logx.As().Error().
			Str("id", s.Info()).
			Str("src", src).
			Str("object", objectName).
			Str("bucket", s.bucketConfig.Bucket).
			Err(err).
			Msg("Failed to upload file to bucket")
		return nil, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	if info.ETag != localChecksum {
		// re-calculate checksum after upload since file might have modified during upload
		latestChecksum, err := fsx.FileMD5(src)
		if err != nil {
			logx.As().Error().
				Str("id", s.Info()).
				Str("src", src).
				Str("objectName", objectName).
				Err(err).
				Msg("Failed to calculate local file checksum")
			return nil, fmt.Errorf("failed to calculate local checksum: %w", err)
		}

		if info.ETag != latestChecksum {
			logx.As().Warn().
				Str("id", s.Info()).
				Str("src", src).
				Str("objectName", objectName).
				Str("expected_md5", latestChecksum).
				Str("actual_md5", info.ETag).
				Msg("Checksum mismatch after upload")

			// Get local file info to compare sizes and log details
			localInfo, err := os.Stat(src)
			if err != nil {
				logx.As().Error().
					Str("id", s.Info()).
					Str("src", src).
					Str("objectName", objectName).
					Err(err).
					Msg("Failed to get local file info")
				return nil, fmt.Errorf("failed to get local file info: %w", err)
			}
			return nil, fmt.Errorf("checksum mismatch after upload: expected %s, got %s "+
				"(file_size_in_bucket = %d, file_size_local = %d)", latestChecksum, info.ETag, info.Size, localInfo.Size())
		}
	}

	logx.As().Info().
		Str("id", s.Info()).
		Str("src", src).
		Str("object", objectName).
		Str("checksum", info.ETag).
		Str("bucket", s.bucketConfig.Bucket).
		Time("last_modified", info.LastModified).
		Str("size", fmt.Sprintf("%d bytes", info.Size)).
		Str("storage_type", s.Type()).
		Str("id", s.Info()).
		Msg("File uploaded successfully to the bucket")

	return &core.UploadInfo{
		Src:          src,
		Dest:         info.Key,
		ChecksumType: "md5",
		Checksum:     info.ETag,
		Size:         info.Size,
		LastModified: info.LastModified,
	}, nil
}

// newS3Handler initializes a new S3 handler with the provided configuration and retry settings.
func newS3Handler(id string, storageType string, bucketConfig config.BucketConfig, retryConfig config.RetryConfig, rootDir string) (*s3Handler, error) {
	if err := config.ValidateBucketConfig(bucketConfig); err != nil {
		logx.As().Error().
			Str("storage_type", storageType).
			Err(err).
			Msg("Invalid bucket configuration")
		return nil, err
	}

	client, err := minio.New(bucketConfig.Endpoint, &minio.Options{
		Creds:      credentials.NewStaticV4(bucketConfig.AccessKey, bucketConfig.SecretKey, ""),
		Secure:     bucketConfig.UseSSL,
		MaxRetries: retryConfig.Limit,
	})
	if err != nil {
		logx.As().Error().
			Str("storage_type", storageType).
			Err(err).
			Msg("Failed to create MinIO client")
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	logx.As().Trace().
		Str("storage_type", storageType).
		Str("endpoint", bucketConfig.Endpoint).
		Msg("MinIO client created successfully")

	s3 := &s3Handler{
		handler: &handler{
			id:          id,
			storageType: storageType,
			pathPrefix:  bucketConfig.Prefix,
			rootDir:     rootDir,
		},
		client:       &minioClientWrapper{client: client},
		bucketConfig: bucketConfig,
		retryConfig:  retryConfig,
		bucketExists: make(map[string]bool),
	}

	s3.handler.syncFile = s3.syncWithBucket

	// create bucket so that multiple goroutines do not compete to create the same bucket
	// try up to 5 minutes rather than failing immediately, as S3 api (minio) may take some time to be ready in a k8s cluster
	err = nil
	ctx := context.Background()
	for i := 0; i < 300; i++ {
		err = s3.ensureBucketExists(ctx)
		if err == nil {
			logx.As().Info().
				Str("storage_type", storageType).
				Int("attempt", i).
				Int("max_attempts", 300).
				Str("bucket", bucketConfig.Bucket).
				Str("storage_type", storageType).
				Str("id", s3.Info()).
				Msg("Bucket exists or created successfully")
			break
		}

		logx.As().Warn().
			Int("attempt", i).
			Int("max_attempts", 300).
			Str("bucket", bucketConfig.Bucket).
			Str("storage_type", storageType).
			Str("id", s3.Info()).
			Err(err).
			Msg("Bucket doesn't exist, trying in 1s...")

		core.ApplyDelay(ctx, time.Second)
	}

	if err != nil {
		return nil, err
	}

	logx.As().Trace().
		Str("id", s3.Info()).
		Str("storage_type", TypeLocalDir).
		Msg("S3 storage handler created successfully")

	return s3, nil
}

// NewS3 creates a new S3 storage handler.
func NewS3(id string, bucketConfig config.BucketConfig, retryConfig config.RetryConfig, rootDir string) (core.Storage, error) {
	return newS3Handler(id, TypeS3, bucketConfig, retryConfig, rootDir)
}

// NewGCSWithS3 creates a new GCS storage handler using the S3-compatible API.
func NewGCSWithS3(id string, bucketConfig config.BucketConfig, retryConfig config.RetryConfig, rootDir string) (core.Storage, error) {
	return newS3Handler(id, TypeGCS, bucketConfig, retryConfig, rootDir)
}
