package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-resty/resty/v2"
)

// DownloadOptions configures the download operation.
type DownloadOptions struct {
	URL          string
	Destination  string // Temp file path
	ExpectedHash string // SHA256 from checksums.txt
	OnProgress   func(downloaded, total int64)
}

// Download downloads a file with checksum verification.
func Download(ctx context.Context, opts DownloadOptions) error {
	// Create a temporary file in the same directory as destination
	// to ensure atomic rename works (same filesystem)
	dir := filepath.Dir(opts.Destination)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, "dagu-download-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up on any error
	defer func() {
		_ = tempFile.Close()
		if _, err := os.Stat(tempPath); err == nil {
			_ = os.Remove(tempPath)
		}
	}()

	// Create HTTP client for download
	client := resty.New().
		SetTimeout(0). // No timeout for downloads
		SetRetryCount(3).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			code := r.StatusCode()
			return code == 429 || (code >= 500 && code <= 504)
		})

	// Download with progress tracking
	resp, err := client.R().
		SetContext(ctx).
		SetOutput(tempPath).
		SetDoNotParseResponse(true).
		Get(opts.URL)

	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode())
	}

	// Close the response body if it exists
	if resp.RawBody() != nil {
		_ = resp.RawBody().Close()
	}

	// Close the temp file before verifying checksum
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify checksum
	if opts.ExpectedHash != "" {
		if err := VerifyChecksum(tempPath, opts.ExpectedHash); err != nil {
			return err
		}
	}

	// Atomically move temp file to destination
	if err := os.Rename(tempPath, opts.Destination); err != nil {
		return fmt.Errorf("failed to move downloaded file: %w", err)
	}

	return nil
}

// VerifyChecksum computes SHA256 and compares with expected hash.
func VerifyChecksum(filePath, expectedHash string) error {
	f, err := os.Open(filePath) //nolint:gosec // filePath is from controlled internal source
	if err != nil {
		return fmt.Errorf("failed to open file for checksum verification: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// FormatBytes formats byte count in human-readable format.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
