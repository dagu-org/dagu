package s3

import (
	"context"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// UploadResult contains the result of an upload operation.
type UploadResult struct {
	Operation         string `json:"operation"`
	Success           bool   `json:"success"`
	Bucket            string `json:"bucket"`
	Key               string `json:"key"`
	URI               string `json:"uri"`
	ETag              string `json:"etag,omitempty"`
	Size              int64  `json:"size"`
	ContentType       string `json:"contentType,omitempty"`
	StorageClass      string `json:"storageClass,omitempty"`
	SSE               string `json:"sse,omitempty"`
	ChecksumAlgorithm string `json:"checksumAlgorithm,omitempty"`
	Duration          string `json:"duration"`
}

func (e *executorImpl) runUpload(ctx context.Context) error {
	start := time.Now()

	// Validate source exists
	sourceInfo, err := os.Stat(e.cfg.Source)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: source file %q does not exist", ErrSourceNotFound, e.cfg.Source)
		}
		return fmt.Errorf("%w: cannot access source file %q: %v", ErrSourceNotFound, e.cfg.Source, err)
	}

	if sourceInfo.IsDir() {
		return fmt.Errorf("%w: source %q is a directory, not a file", ErrConfig, e.cfg.Source)
	}

	// Open source file
	file, err := os.Open(e.cfg.Source)
	if err != nil {
		return fmt.Errorf("%w: failed to open source file: %v", ErrUploadFailed, err)
	}
	defer file.Close()

	// Determine content type
	contentType := e.cfg.ContentType
	if contentType == "" {
		// Try to detect from file extension
		ext := filepath.Ext(e.cfg.Source)
		if ext != "" {
			contentType = mime.TypeByExtension(ext)
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	// Build upload input
	input := &s3.PutObjectInput{
		Bucket:      aws.String(e.cfg.Bucket),
		Key:         aws.String(e.cfg.Key),
		Body:        file,
		ContentType: aws.String(contentType),
	}

	// Set storage class if specified
	if e.cfg.StorageClass != "" {
		input.StorageClass = types.StorageClass(e.cfg.StorageClass)
	}

	// Set metadata if specified
	if len(e.cfg.Metadata) > 0 {
		input.Metadata = e.cfg.Metadata
	}

	// Set server-side encryption
	if e.cfg.ServerSideEncryption != "" {
		input.ServerSideEncryption = types.ServerSideEncryption(e.cfg.ServerSideEncryption)
		if e.cfg.SSEKMSKeyId != "" {
			input.SSEKMSKeyId = aws.String(e.cfg.SSEKMSKeyId)
		}
	}

	// Set ACL
	if e.cfg.ACL != "" {
		input.ACL = types.ObjectCannedACL(e.cfg.ACL)
	}

	// Set tags (URL-encoded query string format)
	if len(e.cfg.Tags) > 0 {
		tagging := buildTagging(e.cfg.Tags)
		input.Tagging = aws.String(tagging)
	}

	// Set checksum algorithm
	if e.cfg.ChecksumAlgorithm != "" {
		input.ChecksumAlgorithm = types.ChecksumAlgorithm(e.cfg.ChecksumAlgorithm)
	}

	// Configure uploader with part size and concurrency
	partSizeBytes := e.cfg.PartSize * 1024 * 1024 // Convert MB to bytes
	if partSizeBytes < manager.MinUploadPartSize {
		partSizeBytes = manager.MinUploadPartSize
	}

	uploader := manager.NewUploader(e.client, func(u *manager.Uploader) {
		u.PartSize = partSizeBytes
		if e.cfg.Concurrency > 0 {
			u.Concurrency = e.cfg.Concurrency
		}
	})

	// Perform the upload
	output, err := uploader.Upload(ctx, input)
	if err != nil {
		classifiedErr := classifyAWSError(err)
		return fmt.Errorf("%w: %v", ErrUploadFailed, classifiedErr)
	}

	// Build result
	result := UploadResult{
		Operation:         opUpload,
		Success:           true,
		Bucket:            e.cfg.Bucket,
		Key:               e.cfg.Key,
		URI:               fmt.Sprintf("s3://%s/%s", e.cfg.Bucket, e.cfg.Key),
		Size:              sourceInfo.Size(),
		ContentType:       contentType,
		StorageClass:      e.cfg.StorageClass,
		SSE:               e.cfg.ServerSideEncryption,
		ChecksumAlgorithm: e.cfg.ChecksumAlgorithm,
		Duration:          time.Since(start).Round(time.Millisecond).String(),
	}

	if output.ETag != nil {
		result.ETag = *output.ETag
	}

	return e.writeResult(result)
}

// buildTagging builds a URL-encoded query string for S3 object tagging.
func buildTagging(tags map[string]string) string {
	var parts []string
	for k, v := range tags {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(parts, "&")
}
