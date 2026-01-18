package s3

import (
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

// Config contains runtime options for the S3 executor.
type Config struct {
	// AWS Connection
	Region          string `mapstructure:"region"`
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"accessKeyId"`
	SecretAccessKey string `mapstructure:"secretAccessKey"`
	SessionToken    string `mapstructure:"sessionToken"`
	Profile         string `mapstructure:"profile"`

	// Path style access (for S3-compatible services like MinIO)
	ForcePathStyle bool `mapstructure:"forcePathStyle"`

	// Common
	Bucket      string `mapstructure:"bucket"`
	Key         string `mapstructure:"key"`
	Source      string `mapstructure:"source"`
	Destination string `mapstructure:"destination"`

	// Upload options
	ContentType  string            `mapstructure:"contentType"`
	StorageClass string            `mapstructure:"storageClass"`
	Metadata     map[string]string `mapstructure:"metadata"`

	// Server-side encryption
	ServerSideEncryption string `mapstructure:"sse"`         // AES256, aws:kms
	SSEKMSKeyId          string `mapstructure:"sseKmsKeyId"` // KMS key ID for aws:kms

	// Access control
	ACL string `mapstructure:"acl"` // private, public-read, etc.

	// Object tagging
	Tags map[string]string `mapstructure:"tags"`

	// Checksum
	ChecksumAlgorithm string `mapstructure:"checksumAlgorithm"` // CRC32, CRC32C, SHA1, SHA256

	// Transfer options
	PartSize    int64 `mapstructure:"partSize"`    // MB, default 10
	Concurrency int   `mapstructure:"concurrency"` // default 5

	// List options
	Prefix       string `mapstructure:"prefix"`
	Delimiter    string `mapstructure:"delimiter"`
	MaxKeys      int    `mapstructure:"maxKeys"`
	Recursive    bool   `mapstructure:"recursive"`
	OutputFormat string `mapstructure:"outputFormat"` // json, jsonl

	// Delete options
	Quiet bool `mapstructure:"quiet"` // Suppress output for delete

	// Advanced options
	DisableSSL bool `mapstructure:"disableSSL"` // For local testing only
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		PartSize:     10, // 10 MB
		Concurrency:  5,
		MaxKeys:      1000,
		OutputFormat: "json",
	}
}

// decodeConfig decodes the raw config map into a Config struct.
func decodeConfig(raw map[string]any, cfg *Config) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		TagName:          "mapstructure",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(raw)
}

// ValidateForOperation validates the config for the given operation.
func (c *Config) ValidateForOperation(operation string) error {
	if c == nil {
		return fmt.Errorf("%w: missing configuration", ErrConfig)
	}

	// Bucket is required for all operations
	if c.Bucket == "" {
		return fmt.Errorf("%w: bucket is required", ErrConfig)
	}

	switch operation {
	case opUpload:
		if c.Source == "" {
			return fmt.Errorf("%w: source is required for upload", ErrConfig)
		}
		if c.Key == "" {
			return fmt.Errorf("%w: key is required for upload", ErrConfig)
		}
	case opDownload:
		if c.Key == "" {
			return fmt.Errorf("%w: key is required for download", ErrConfig)
		}
		if c.Destination == "" {
			return fmt.Errorf("%w: destination is required for download", ErrConfig)
		}
	case opList:
		// List only needs bucket (already validated)
	case opDelete:
		// Delete needs either key or prefix
		if c.Key == "" && c.Prefix == "" {
			return fmt.Errorf("%w: key or prefix is required for delete", ErrConfig)
		}
	default:
		return fmt.Errorf("%w: unknown operation %q", ErrConfig, operation)
	}

	// Validate storage class if provided
	if c.StorageClass != "" {
		validStorageClasses := []string{
			"STANDARD", "REDUCED_REDUNDANCY", "STANDARD_IA",
			"ONEZONE_IA", "INTELLIGENT_TIERING", "GLACIER",
			"DEEP_ARCHIVE", "GLACIER_IR",
		}
		found := false
		for _, sc := range validStorageClasses {
			if strings.EqualFold(c.StorageClass, sc) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: invalid storage class %q", ErrConfig, c.StorageClass)
		}
	}

	// Validate output format
	if c.OutputFormat != "" && c.OutputFormat != "json" && c.OutputFormat != "jsonl" {
		return fmt.Errorf("%w: outputFormat must be 'json' or 'jsonl'", ErrConfig)
	}

	// Validate server-side encryption
	if c.ServerSideEncryption != "" {
		validSSE := []string{"AES256", "aws:kms"}
		found := false
		for _, sse := range validSSE {
			if strings.EqualFold(c.ServerSideEncryption, sse) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: sse must be 'AES256' or 'aws:kms'", ErrConfig)
		}
		// KMS key ID is required for aws:kms
		if strings.EqualFold(c.ServerSideEncryption, "aws:kms") && c.SSEKMSKeyId == "" {
			return fmt.Errorf("%w: sseKmsKeyId is required when sse is 'aws:kms'", ErrConfig)
		}
	}

	// Validate ACL
	if c.ACL != "" {
		validACLs := []string{
			"private", "public-read", "public-read-write",
			"authenticated-read", "aws-exec-read", "bucket-owner-read",
			"bucket-owner-full-control",
		}
		found := false
		for _, acl := range validACLs {
			if strings.EqualFold(c.ACL, acl) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: invalid acl %q", ErrConfig, c.ACL)
		}
	}

	// Validate checksum algorithm
	if c.ChecksumAlgorithm != "" {
		validChecksums := []string{"CRC32", "CRC32C", "SHA1", "SHA256"}
		found := false
		for _, cs := range validChecksums {
			if strings.EqualFold(c.ChecksumAlgorithm, cs) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: checksumAlgorithm must be one of: CRC32, CRC32C, SHA1, SHA256", ErrConfig)
		}
	}

	// Validate concurrency
	if c.Concurrency < 0 {
		return fmt.Errorf("%w: concurrency must be >= 0", ErrConfig)
	}

	// Validate part size
	if c.PartSize < 0 {
		return fmt.Errorf("%w: partSize must be >= 0", ErrConfig)
	}

	return nil
}

func ptrFloat(f float64) *float64 { return &f }

var configSchema = &jsonschema.Schema{
	Type:     "object",
	Required: []string{"bucket"},
	Properties: map[string]*jsonschema.Schema{
		// AWS Connection
		"region":          {Type: "string", Description: "AWS region (e.g., us-east-1)"},
		"endpoint":        {Type: "string", Description: "Custom S3-compatible endpoint URL"},
		"accessKeyId":     {Type: "string", Description: "AWS access key ID"},
		"secretAccessKey": {Type: "string", Description: "AWS secret access key"},
		"sessionToken":    {Type: "string", Description: "AWS session token (for temporary credentials)"},
		"profile":         {Type: "string", Description: "AWS credentials profile name"},
		"forcePathStyle":  {Type: "boolean", Description: "Use path-style addressing (for S3-compatible services)"},

		// Common
		"bucket":      {Type: "string", Description: "S3 bucket name (required)"},
		"key":         {Type: "string", Description: "S3 object key (required for upload/download/delete)"},
		"source":      {Type: "string", Description: "Local file path to upload (required for upload)"},
		"destination": {Type: "string", Description: "Local file path for download (required for download)"},

		// Upload options
		"contentType":  {Type: "string", Description: "Content-Type for the uploaded object"},
		"storageClass": {Type: "string", Description: "Storage class (STANDARD, STANDARD_IA, etc.)"},
		"metadata":     {Type: "object", Description: "Custom metadata key-value pairs"},

		// Server-side encryption
		"sse":         {Type: "string", Enum: []any{"AES256", "aws:kms"}, Description: "Server-side encryption type"},
		"sseKmsKeyId": {Type: "string", Description: "KMS key ID (required when sse is 'aws:kms')"},

		// Access control
		"acl": {Type: "string", Description: "Canned ACL (private, public-read, etc.)"},

		// Object tagging
		"tags": {Type: "object", Description: "Object tags as key-value pairs"},

		// Checksum
		"checksumAlgorithm": {Type: "string", Enum: []any{"CRC32", "CRC32C", "SHA1", "SHA256"}, Description: "Checksum algorithm for data integrity"},

		// Transfer options
		"partSize":    {Type: "integer", Minimum: ptrFloat(5), Description: "Multipart upload part size in MB (default: 10, min: 5)"},
		"concurrency": {Type: "integer", Minimum: ptrFloat(1), Description: "Number of concurrent upload/download parts (default: 5)"},

		// List options
		"prefix":       {Type: "string", Description: "Filter objects by key prefix"},
		"delimiter":    {Type: "string", Description: "Delimiter for grouping keys (e.g., '/')"},
		"maxKeys":      {Type: "integer", Minimum: ptrFloat(1), Description: "Maximum number of keys to return (default: 1000)"},
		"recursive":    {Type: "boolean", Description: "List all objects recursively (ignores delimiter)"},
		"outputFormat": {Type: "string", Enum: []any{"json", "jsonl"}, Description: "Output format: json (default) or jsonl"},

		// Delete options
		"quiet": {Type: "boolean", Description: "Suppress output for delete operation"},

		// Advanced
		"disableSSL": {Type: "boolean", Description: "Disable SSL (for local testing only)"},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("s3", configSchema)
}
