package s3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Sentinel errors for S3 operations.
var (
	ErrConfig         = errors.New("s3: configuration error")
	ErrSourceNotFound = errors.New("s3: source not found")
	ErrBucketNotFound = errors.New("s3: bucket not found")
	ErrObjectNotFound = errors.New("s3: object not found")
	ErrPermission     = errors.New("s3: permission denied")
	ErrUploadFailed   = errors.New("s3: upload failed")
	ErrDownloadFailed = errors.New("s3: download failed")
	ErrListFailed     = errors.New("s3: list failed")
	ErrDeleteFailed   = errors.New("s3: delete failed")
	ErrNetwork        = errors.New("s3: network error")
	ErrTimeout        = errors.New("s3: operation timeout")
	ErrCredentials    = errors.New("s3: invalid credentials")
	ErrInvalidBucket  = errors.New("s3: invalid bucket name")
	ErrInvalidKey     = errors.New("s3: invalid object key")
)

// classifyAWSError converts AWS SDK errors into sentinel errors.
func classifyAWSError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// Check for S3 specific errors
	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return fmt.Errorf("%w: %v", ErrBucketNotFound, err)
	}

	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
	}

	// Check for Smithy API errors
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch {
		case code == "AccessDenied" || code == "Forbidden" || strings.Contains(code, "AccessDenied"):
			return fmt.Errorf("%w: %v", ErrPermission, err)
		case code == "InvalidAccessKeyId" || code == "SignatureDoesNotMatch":
			return fmt.Errorf("%w: %v", ErrCredentials, err)
		case code == "NoSuchBucket" || code == "BucketNotFound":
			return fmt.Errorf("%w: %v", ErrBucketNotFound, err)
		case code == "NoSuchKey" || code == "NotFound":
			return fmt.Errorf("%w: %v", ErrObjectNotFound, err)
		case code == "InvalidBucketName":
			return fmt.Errorf("%w: %v", ErrInvalidBucket, err)
		case code == "KeyTooLongError":
			return fmt.Errorf("%w: %v", ErrInvalidKey, err)
		case code == "RequestTimeout" || code == "SlowDown":
			return fmt.Errorf("%w: %v", ErrTimeout, err)
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return fmt.Errorf("%w: %v", ErrNetwork, err)
	}

	return err
}

// exitCodeFor returns an appropriate exit code for the given error.
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	switch {
	case errors.Is(err, ErrConfig):
		return 2
	case errors.Is(err, ErrSourceNotFound):
		return 3
	case errors.Is(err, ErrBucketNotFound):
		return 4
	case errors.Is(err, ErrObjectNotFound):
		return 5
	case errors.Is(err, ErrPermission):
		return 6
	case errors.Is(err, ErrCredentials):
		return 7
	case errors.Is(err, ErrUploadFailed):
		return 8
	case errors.Is(err, ErrDownloadFailed):
		return 9
	case errors.Is(err, ErrListFailed):
		return 10
	case errors.Is(err, ErrDeleteFailed):
		return 11
	case errors.Is(err, ErrNetwork):
		return 12
	case errors.Is(err, ErrTimeout):
		return 13
	case errors.Is(err, context.Canceled):
		return 14
	case errors.Is(err, context.DeadlineExceeded):
		return 15
	default:
		return 1
	}
}

// encodeJSON writes v as indented JSON to w with a trailing newline.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

