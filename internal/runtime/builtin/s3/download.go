package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
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

	// Get object metadata first to check existence and get info
	objInfo, err := e.client.StatObject(ctx, e.cfg.Bucket, e.cfg.Key, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	// Download to temp file first (atomic)
	tmpFile := e.cfg.Destination + ".tmp"

	// Clean up temp file on error
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	// Download file using FGetObject (handles large files automatically)
	err = e.client.FGetObject(ctx, e.cfg.Bucket, e.cfg.Key, tmpFile, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	// Atomic rename to final destination
	if err := os.Rename(tmpFile, e.cfg.Destination); err != nil {
		return fmt.Errorf("%w: failed to move file to destination: %v", ErrDownloadFailed, err)
	}

	// Get final file info
	fileInfo, err := os.Stat(e.cfg.Destination)
	if err != nil {
		return fmt.Errorf("%w: failed to stat destination file: %v", ErrDownloadFailed, err)
	}

	// Build result
	result := DownloadResult{
		Operation:   opDownload,
		Success:     true,
		Bucket:      e.cfg.Bucket,
		Key:         e.cfg.Key,
		URI:         fmt.Sprintf("s3://%s/%s", e.cfg.Bucket, e.cfg.Key),
		Destination: e.cfg.Destination,
		Size:        fileInfo.Size(),
		ContentType: objInfo.ContentType,
		ETag:        objInfo.ETag,
		Duration:    time.Since(start).Round(time.Millisecond).String(),
	}

	return e.writeResult(result)
}
