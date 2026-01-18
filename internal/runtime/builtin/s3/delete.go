package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// DeleteResult contains the result of a delete operation.
type DeleteResult struct {
	Operation    string   `json:"operation"`
	Success      bool     `json:"success"`
	Bucket       string   `json:"bucket"`
	Key          string   `json:"key,omitempty"`
	Prefix       string   `json:"prefix,omitempty"`
	DeletedCount int      `json:"deletedCount"`
	DeletedKeys  []string `json:"deletedKeys,omitempty"`
	ErrorCount   int      `json:"errorCount,omitempty"`
	Errors       []string `json:"errors,omitempty"`
	Duration     string   `json:"duration"`
}

func (e *executorImpl) runDelete(ctx context.Context) error {
	start := time.Now()

	// Single key delete
	if e.cfg.Key != "" {
		return e.deleteSingleObject(ctx, start)
	}

	// Batch delete by prefix
	return e.deleteByPrefix(ctx, start)
}

func (e *executorImpl) deleteSingleObject(ctx context.Context, start time.Time) error {
	err := e.client.RemoveObject(ctx, e.cfg.Bucket, e.cfg.Key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}

	if e.cfg.Quiet {
		return nil
	}

	result := DeleteResult{
		Operation:    opDelete,
		Success:      true,
		Bucket:       e.cfg.Bucket,
		Key:          e.cfg.Key,
		DeletedCount: 1,
		DeletedKeys:  []string{e.cfg.Key},
		Duration:     time.Since(start).Round(time.Millisecond).String(),
	}

	return e.writeResult(result)
}

func (e *executorImpl) deleteByPrefix(ctx context.Context, start time.Time) error {
	var deletedKeys []string
	var deleteErrors []string

	opts := minio.ListObjectsOptions{
		Prefix:    e.cfg.Prefix,
		Recursive: true,
	}

	for object := range e.client.ListObjects(ctx, e.cfg.Bucket, opts) {
		if object.Err != nil {
			return fmt.Errorf("%w: failed to list objects: %v", ErrDeleteFailed, object.Err)
		}
		if err := e.client.RemoveObject(ctx, e.cfg.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			deleteErrors = append(deleteErrors, fmt.Sprintf("%s: %v", object.Key, err))
		} else {
			deletedKeys = append(deletedKeys, object.Key)
		}
	}

	hasErrors := len(deleteErrors) > 0

	// Write result unless quiet mode with no errors
	if !e.cfg.Quiet || hasErrors {
		result := DeleteResult{
			Operation:    opDelete,
			Success:      !hasErrors,
			Bucket:       e.cfg.Bucket,
			Prefix:       e.cfg.Prefix,
			DeletedCount: len(deletedKeys),
			ErrorCount:   len(deleteErrors),
			Duration:     time.Since(start).Round(time.Millisecond).String(),
		}
		// Only include deleted keys if not too many
		if len(deletedKeys) <= 100 {
			result.DeletedKeys = deletedKeys
		}
		if hasErrors {
			result.Errors = deleteErrors
		}
		if err := e.writeResult(result); err != nil {
			return err
		}
	}

	if hasErrors {
		return fmt.Errorf("%w: %d of %d objects failed to delete",
			ErrDeleteFailed, len(deleteErrors), len(deletedKeys)+len(deleteErrors))
	}

	return nil
}
