package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	_, err := e.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(e.cfg.Bucket),
		Key:    aws.String(e.cfg.Key),
	})
	if err != nil {
		classifiedErr := classifyAWSError(err)
		return fmt.Errorf("%w: %v", ErrDeleteFailed, classifiedErr)
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

	// List objects with the prefix
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(e.cfg.Bucket),
		Prefix: aws.String(e.cfg.Prefix),
	}

	paginator := s3.NewListObjectsV2Paginator(e.client, input)

	// Collect keys to delete in batches of 1000 (S3 limit)
	const batchSize = 1000
	var keysToDelete []types.ObjectIdentifier

	for paginator.HasMorePages() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		page, err := paginator.NextPage(ctx)
		if err != nil {
			classifiedErr := classifyAWSError(err)
			return fmt.Errorf("%w: failed to list objects: %v", ErrDeleteFailed, classifiedErr)
		}

		for _, obj := range page.Contents {
			keysToDelete = append(keysToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})

			// Delete in batches
			if len(keysToDelete) >= batchSize {
				deleted, errors := e.deleteBatch(ctx, keysToDelete)
				deletedKeys = append(deletedKeys, deleted...)
				deleteErrors = append(deleteErrors, errors...)
				keysToDelete = keysToDelete[:0]
			}
		}
	}

	// Delete remaining objects
	if len(keysToDelete) > 0 {
		deleted, errors := e.deleteBatch(ctx, keysToDelete)
		deletedKeys = append(deletedKeys, deleted...)
		deleteErrors = append(deleteErrors, errors...)
	}

	if e.cfg.Quiet {
		return nil
	}

	result := DeleteResult{
		Operation:    opDelete,
		Success:      len(deleteErrors) == 0,
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
	if len(deleteErrors) > 0 {
		result.Errors = deleteErrors
	}

	return e.writeResult(result)
}

func (e *executorImpl) deleteBatch(ctx context.Context, objects []types.ObjectIdentifier) (deleted []string, errors []string) {
	if len(objects) == 0 {
		return nil, nil
	}

	output, err := e.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(e.cfg.Bucket),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	})
	if err != nil {
		// If the entire request fails, report all keys as errors
		for _, obj := range objects {
			errors = append(errors, fmt.Sprintf("%s: %v", aws.ToString(obj.Key), err))
		}
		return nil, errors
	}

	// Collect successful deletes
	for _, d := range output.Deleted {
		if d.Key != nil {
			deleted = append(deleted, *d.Key)
		}
	}

	// Collect errors
	for _, e := range output.Errors {
		errMsg := fmt.Sprintf("%s: %s", aws.ToString(e.Key), aws.ToString(e.Message))
		errors = append(errors, errMsg)
	}

	return deleted, errors
}
