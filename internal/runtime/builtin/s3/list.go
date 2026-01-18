package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	Operation    string       `json:"operation"`
	Success      bool         `json:"success"`
	Bucket       string       `json:"bucket"`
	Prefix       string       `json:"prefix,omitempty"`
	Objects      []ListObject `json:"objects"`
	TotalCount   int          `json:"totalCount"`
	IsTruncated  bool         `json:"isTruncated"`
	NextMarker   string       `json:"nextMarker,omitempty"`
	CommonPrefix []string     `json:"commonPrefixes,omitempty"`
	Duration     string       `json:"duration"`
}

func (e *executorImpl) runList(ctx context.Context) error {
	start := time.Now()

	var objects []ListObject
	var commonPrefixes []string
	var nextMarker string
	var isTruncated bool

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(e.cfg.Bucket),
	}

	// Set prefix if provided
	if e.cfg.Prefix != "" {
		input.Prefix = aws.String(e.cfg.Prefix)
	}

	// Set delimiter unless recursive mode is enabled
	if !e.cfg.Recursive {
		delimiter := e.cfg.Delimiter
		if delimiter == "" {
			delimiter = "/" // Default delimiter for hierarchical listing
		}
		input.Delimiter = aws.String(delimiter)
	}

	// Set max keys
	if e.cfg.MaxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(e.cfg.MaxKeys))
	}

	// Use paginator for handling large results
	paginator := s3.NewListObjectsV2Paginator(e.client, input)

	totalCount := 0
	maxObjects := e.cfg.MaxKeys
	if maxObjects <= 0 {
		maxObjects = 1000 // Default limit
	}

	// JSONL mode: stream objects one by one
	if e.cfg.OutputFormat == "jsonl" {
		for paginator.HasMorePages() {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			page, err := paginator.NextPage(ctx)
			if err != nil {
				classifiedErr := classifyAWSError(err)
				return fmt.Errorf("%w: %v", ErrListFailed, classifiedErr)
			}

			for _, obj := range page.Contents {
				if totalCount >= maxObjects {
					break
				}

				listObj := ListObject{
					Key:  aws.ToString(obj.Key),
					Size: aws.ToInt64(obj.Size),
				}
				if obj.LastModified != nil {
					listObj.LastModified = obj.LastModified.Format(time.RFC3339)
				}
				if obj.ETag != nil {
					listObj.ETag = *obj.ETag
				}
				if obj.StorageClass != "" {
					listObj.StorageClass = string(obj.StorageClass)
				}

				if err := encodeJSON(e.stdout, listObj); err != nil {
					return fmt.Errorf("%w: failed to write output: %v", ErrListFailed, err)
				}
				totalCount++
			}

			if totalCount >= maxObjects {
				break
			}
		}
		return nil
	}

	// JSON mode: collect all objects first
	for paginator.HasMorePages() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		page, err := paginator.NextPage(ctx)
		if err != nil {
			classifiedErr := classifyAWSError(err)
			return fmt.Errorf("%w: %v", ErrListFailed, classifiedErr)
		}

		for _, obj := range page.Contents {
			if totalCount >= maxObjects {
				isTruncated = true
				break
			}

			listObj := ListObject{
				Key:  aws.ToString(obj.Key),
				Size: aws.ToInt64(obj.Size),
			}
			if obj.LastModified != nil {
				listObj.LastModified = obj.LastModified.Format(time.RFC3339)
			}
			if obj.ETag != nil {
				listObj.ETag = *obj.ETag
			}
			if obj.StorageClass != "" {
				listObj.StorageClass = string(obj.StorageClass)
			}

			objects = append(objects, listObj)
			totalCount++
		}

		// Collect common prefixes (for non-recursive listing)
		for _, prefix := range page.CommonPrefixes {
			if prefix.Prefix != nil {
				commonPrefixes = append(commonPrefixes, *prefix.Prefix)
			}
		}

		// Check if we should continue
		if totalCount >= maxObjects {
			if page.NextContinuationToken != nil {
				nextMarker = *page.NextContinuationToken
				isTruncated = true
			}
			break
		}

		// Track continuation for truncated results
		if page.IsTruncated != nil && *page.IsTruncated {
			if page.NextContinuationToken != nil {
				nextMarker = *page.NextContinuationToken
			}
			isTruncated = true
		}
	}

	result := ListResult{
		Operation:    opList,
		Success:      true,
		Bucket:       e.cfg.Bucket,
		Prefix:       e.cfg.Prefix,
		Objects:      objects,
		TotalCount:   totalCount,
		IsTruncated:  isTruncated,
		NextMarker:   nextMarker,
		CommonPrefix: commonPrefixes,
		Duration:     time.Since(start).Round(time.Millisecond).String(),
	}

	return e.writeResult(result)
}
