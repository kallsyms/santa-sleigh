package uploader

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/kallsyms/santa-sleigh/internal/config"
)

// Uploader defines the behaviour required by the daemon for object storage.
type Uploader interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64) error
}

// S3Uploader streams files to an S3-compatible endpoint.
type S3Uploader struct {
	bucket string
	upload *manager.Uploader
	cfg    config.AWSConfig
}

// NewS3Uploader constructs an S3-backed uploader using the provided configuration.
func NewS3Uploader(ctx context.Context, awsCfg config.AWSConfig, maxRetries int) (*S3Uploader, error) {
	if awsCfg.Region == "" {
		return nil, fmt.Errorf("aws region is required")
	}
	if awsCfg.Bucket == "" {
		return nil, fmt.Errorf("aws bucket is required")
	}

	loadOpts := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(awsCfg.Region),
	}
	if awsCfg.Profile != "" {
		loadOpts = append(loadOpts, awscfg.WithSharedConfigProfile(awsCfg.Profile))
	}
	if awsCfg.AccessKey != "" && awsCfg.SecretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(awsCfg.AccessKey, awsCfg.SecretKey, awsCfg.SessionToken)
		loadOpts = append(loadOpts, awscfg.WithCredentialsProvider(creds))
	}

	cfg, err := awscfg.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	if maxRetries > 0 {
		cfg.RetryMaxAttempts = maxRetries
	}

	var clientOpts []func(*s3.Options)
	if awsCfg.CustomURL != "" {
		endpointResolver := s3.EndpointResolverFromURL(awsCfg.CustomURL)
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.Region = awsCfg.Region
			o.EndpointResolver = endpointResolver
		})
	}
	if awsCfg.UsePathStyle {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, clientOpts...)
	uploader := manager.NewUploader(client)

	return &S3Uploader{
		bucket: awsCfg.Bucket,
		upload: uploader,
		cfg:    awsCfg,
	}, nil
}

// Upload streams the file body to S3 and returns when the transfer completes.
func (s *S3Uploader) Upload(ctx context.Context, key string, body io.Reader, size int64) error {
	if key == "" {
		return fmt.Errorf("upload key must not be empty")
	}
	if body == nil {
		return fmt.Errorf("upload body must not be nil")
	}

	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   body,
		ACL:    types.ObjectCannedACLPrivate,
	}

	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := s.upload.Upload(ctx, input)
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}
