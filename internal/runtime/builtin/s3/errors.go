package s3

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	ErrCredentials    = errors.New("s3: invalid credentials")
)

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
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return 12
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

