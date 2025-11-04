package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	ErrConfig          = errors.New("archive: configuration error")
	ErrSourceNotFound  = errors.New("archive: source not found")
	ErrDestination     = errors.New("archive: destination error")
	ErrFormatDetection = errors.New("archive: format detection failed")
	ErrExtract         = errors.New("archive: extraction failed")
	ErrCreate          = errors.New("archive: creation failed")
	ErrCompress        = errors.New("archive: compression failed")
	ErrDecompress      = errors.New("archive: decompression failed")
	ErrPermission      = errors.New("archive: permission denied")
	ErrDiskSpace       = errors.New("archive: insufficient disk space")
	ErrCorrupted       = errors.New("archive: corrupted archive")
	ErrPassword        = errors.New("archive: password error")
)

func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func wrapError(kind error, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", kind, err)
}
