package s3

import (
	"context"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// createClient creates a MinIO client based on the configuration.
// MinIO client supports AWS S3 and all S3-compatible services (GCS, MinIO, etc.).
func createClient(_ context.Context, cfg *Config) (*minio.Client, error) {
	endpoint, secure := parseEndpoint(cfg)

	// Build credentials provider with proper chain
	var creds *credentials.Credentials
	switch {
	case cfg.AccessKeyID != "" && cfg.SecretAccessKey != "":
		// Explicit credentials provided
		creds = credentials.NewStaticV4(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
	case cfg.Profile != "":
		// Use specific AWS profile
		creds = credentials.NewFileAWSCredentials("", cfg.Profile)
	default:
		// Use credentials chain: env vars -> shared config -> IAM
		creds = credentials.NewChainCredentials([]credentials.Provider{
			&credentials.EnvAWS{},
			&credentials.FileAWSCredentials{},
			&credentials.IAM{},
		})
	}

	opts := &minio.Options{
		Creds:  creds,
		Secure: secure,
	}

	if cfg.Region != "" {
		opts.Region = cfg.Region
	}

	// Enable path-style addressing for S3-compatible services (MinIO, LocalStack, etc.)
	if cfg.ForcePathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}

	return minio.New(endpoint, opts)
}

// parseEndpoint parses the endpoint configuration and returns the host and secure flag.
func parseEndpoint(cfg *Config) (endpoint string, secure bool) {
	if cfg.Endpoint == "" {
		// Default to AWS S3
		if cfg.Region != "" {
			return "s3." + cfg.Region + ".amazonaws.com", true
		}
		return "s3.amazonaws.com", true
	}

	endpoint = cfg.Endpoint
	secure = !cfg.DisableSSL

	// Strip scheme if present and determine secure from scheme
	if after, ok := strings.CutPrefix(endpoint, "https://"); ok {
		endpoint = after
		secure = true
	} else if after, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint = after
		secure = false
	}

	return endpoint, secure
}
