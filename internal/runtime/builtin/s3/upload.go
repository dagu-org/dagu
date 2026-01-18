package s3

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
)

// UploadResult contains the result of an upload operation.
type UploadResult struct {
	Operation    string `json:"operation"`
	Success      bool   `json:"success"`
	Bucket       string `json:"bucket"`
	Key          string `json:"key"`
	URI          string `json:"uri"`
	ETag         string `json:"etag,omitempty"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
	Duration     string `json:"duration"`
}

func (e *executorImpl) runUpload(ctx context.Context) error {
	start := time.Now()

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

	contentType := e.resolveContentType()

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: e.cfg.Metadata,
		StorageClass: e.cfg.StorageClass,
		UserTags:     e.cfg.Tags,
	}

	info, err := e.client.FPutObject(ctx, e.cfg.Bucket, e.cfg.Key, e.cfg.Source, opts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}

	return e.writeResult(UploadResult{
		Operation:    opUpload,
		Success:      true,
		Bucket:       e.cfg.Bucket,
		Key:          e.cfg.Key,
		URI:          fmt.Sprintf("s3://%s/%s", e.cfg.Bucket, e.cfg.Key),
		ETag:         info.ETag,
		Size:         info.Size,
		ContentType:  contentType,
		StorageClass: e.cfg.StorageClass,
		Duration:     time.Since(start).Round(time.Millisecond).String(),
	})
}

// resolveContentType determines the content type for the upload.
// Uses configured content type if set, otherwise detects from file extension.
func (e *executorImpl) resolveContentType() string {
	if e.cfg.ContentType != "" {
		return e.cfg.ContentType
	}
	if ext := filepath.Ext(e.cfg.Source); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	return "application/octet-stream"
}
