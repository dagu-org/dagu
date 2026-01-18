package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DownloadResult contains the result of a download operation.
type DownloadResult struct {
	Operation   string `json:"operation"`
	Success     bool   `json:"success"`
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	URI         string `json:"uri"`
	Destination string `json:"destination"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	ETag        string `json:"etag,omitempty"`
	Duration    string `json:"duration"`
}

func (e *executorImpl) runDownload(ctx context.Context) error {
	start := time.Now()

	// Ensure destination directory exists
	destDir := filepath.Dir(e.cfg.Destination)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("%w: failed to create destination directory: %v", ErrDownloadFailed, err)
	}

	// Get object metadata first to check existence and get size
	headOutput, err := e.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(e.cfg.Bucket),
		Key:    aws.String(e.cfg.Key),
	})
	if err != nil {
		classifiedErr := classifyAWSError(err)
		return fmt.Errorf("%w: %v", ErrDownloadFailed, classifiedErr)
	}

	// Create a temporary file for atomic download
	tempFile, err := os.CreateTemp(destDir, ".s3download-*")
	if err != nil {
		return fmt.Errorf("%w: failed to create temp file: %v", ErrDownloadFailed, err)
	}
	tempPath := tempFile.Name()

	// Clean up on failure
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	// Configure downloader with part size and concurrency
	partSizeBytes := e.cfg.PartSize * 1024 * 1024 // Convert MB to bytes
	if partSizeBytes < manager.MinUploadPartSize {
		partSizeBytes = manager.MinUploadPartSize
	}

	downloader := manager.NewDownloader(e.client, func(d *manager.Downloader) {
		d.PartSize = partSizeBytes
		if e.cfg.Concurrency > 0 {
			d.Concurrency = e.cfg.Concurrency
		}
	})

	// Download to temp file
	numBytes, err := downloader.Download(ctx, tempFile, &s3.GetObjectInput{
		Bucket: aws.String(e.cfg.Bucket),
		Key:    aws.String(e.cfg.Key),
	})
	if err != nil {
		_ = tempFile.Close()
		classifiedErr := classifyAWSError(err)
		return fmt.Errorf("%w: %v", ErrDownloadFailed, classifiedErr)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("%w: failed to close temp file: %v", ErrDownloadFailed, err)
	}

	// Atomic rename to final destination
	if err := os.Rename(tempPath, e.cfg.Destination); err != nil {
		return fmt.Errorf("%w: failed to move file to destination: %v", ErrDownloadFailed, err)
	}
	success = true

	// Build result
	result := DownloadResult{
		Operation:   opDownload,
		Success:     true,
		Bucket:      e.cfg.Bucket,
		Key:         e.cfg.Key,
		URI:         fmt.Sprintf("s3://%s/%s", e.cfg.Bucket, e.cfg.Key),
		Destination: e.cfg.Destination,
		Size:        numBytes,
		Duration:    time.Since(start).Round(time.Millisecond).String(),
	}

	if headOutput.ContentType != nil {
		result.ContentType = *headOutput.ContentType
	}
	if headOutput.ETag != nil {
		result.ETag = *headOutput.ETag
	}

	return e.writeResult(result)
}
