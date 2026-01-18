package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
)

// ListObject represents a single S3 object in list results.
type ListObject struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"etag,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

// ListResult contains the result of a list operation.
type ListResult struct {
	Operation  string       `json:"operation"`
	Success    bool         `json:"success"`
	Bucket     string       `json:"bucket"`
	Prefix     string       `json:"prefix,omitempty"`
	Objects    []ListObject `json:"objects"`
	TotalCount int          `json:"totalCount"`
	Duration   string       `json:"duration"`
}

func (e *executorImpl) runList(ctx context.Context) error {
	start := time.Now()

	opts := minio.ListObjectsOptions{
		Prefix:    e.cfg.Prefix,
		Recursive: e.cfg.Recursive,
	}

	maxObjects := e.cfg.MaxKeys
	if maxObjects <= 0 {
		maxObjects = 1000
	}

	streamMode := e.cfg.OutputFormat == "jsonl"
	var objects []ListObject
	count := 0

	for object := range e.client.ListObjects(ctx, e.cfg.Bucket, opts) {
		if object.Err != nil {
			return fmt.Errorf("%w: %v", ErrListFailed, object.Err)
		}

		if count >= maxObjects {
			break
		}
		count++

		listObj := ListObject{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified.Format(time.RFC3339),
			ETag:         object.ETag,
			StorageClass: object.StorageClass,
		}

		if streamMode {
			if err := encodeJSON(e.stdout, listObj); err != nil {
				return fmt.Errorf("%w: failed to write output: %v", ErrListFailed, err)
			}
		} else {
			objects = append(objects, listObj)
		}
	}

	if streamMode {
		return nil
	}

	result := ListResult{
		Operation:  opList,
		Success:    true,
		Bucket:     e.cfg.Bucket,
		Prefix:     e.cfg.Prefix,
		Objects:    objects,
		TotalCount: len(objects),
		Duration:   time.Since(start).Round(time.Millisecond).String(),
	}

	return e.writeResult(result)
}
