package s3

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// createClient creates an S3 client based on the configuration.
// It supports the standard AWS credential chain plus explicit credentials.
func createClient(ctx context.Context, cfg *Config) (*s3.Client, error) {
	var opts []func(*config.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		creds := credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
		opts = append(opts, config.WithCredentialsProvider(creds))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load AWS config: %v", ErrConfig, err)
	}

	// Build S3 client options
	var s3Opts []func(*s3.Options)

	// Set custom endpoint for S3-compatible services (MinIO, DigitalOcean Spaces, etc.)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	if cfg.ForcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	if cfg.DisableSSL {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.HTTPClient = &http.Client{
				Transport: &http.Transport{
					// Use default transport settings
				},
			}
			// Override endpoint scheme to http if needed
			if cfg.Endpoint != "" {
				o.EndpointOptions.DisableHTTPS = true
			}
		})
	}

	// Create and return the S3 client
	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return client, nil
}
