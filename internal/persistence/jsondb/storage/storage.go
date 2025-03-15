package storage

import (
	"context"
	"regexp"
	"runtime"
	"time"
)

var maxWorkers = runtime.NumCPU()

// dataFileExtension is the file extension for history record files.
var dataFileExtension = ".dat"

// rTimestamp is a regular expression to match the timestamp in the file name.
var rTimestamp = regexp.MustCompile(`2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}|2\d{7}\.\d{2}:\d{2}:\d{2}\.\d{3}Z`)

// Filename formats
const (
	requestIDLenSafe  = 8
	dateTimeFormatUTC = "20060102.15:04:05.000Z"
	dateTimeFormat    = "20060102.15:04:05.000"
	dateFormat        = "20060102"
)

// TimeInUTC is a wrapper for time.Time that ensures the time is in UTC.
type TimeInUTC struct{ time.Time }

// NewUTC creates a new timeInUTC from a time.Time.
func NewUTC(t time.Time) TimeInUTC {
	return TimeInUTC{t.UTC()}
}

type Storage interface {
	// Latest returns the latest history record files for the specified address, up to itemLimit.
	Latest(ctx context.Context, a Address, itemLimit int) []string
	// LatestAfter returns the path to the latest history record file.
	LatestAfter(ctx context.Context, a Address, cutoff TimeInUTC) (string, error)
	// GenerateFilePath generates a file path for a history record.
	GenerateFilePath(ctx context.Context, a Address, timestamp TimeInUTC, reqID string) string
	// Rename renames a file from the old address to the new address.
	Rename(ctx context.Context, o, n Address) error
	// RemoveOld removes history records older than retentionDays.
	// It uses parallel processing for improved performance with large datasets.
	RemoveOld(ctx context.Context, a Address, retentionDays int) error
	// FindByRequestID finds a history record by request ID.
	// It returns the most recent record if multiple matches exist.
	FindByRequestID(ctx context.Context, a Address, requestID string) (string, error)
}

var _ Storage = (*storage)(nil)

type storage struct {
	legacy Storage
}

func New() Storage {
	return &storage{legacy: &LegacyStorage{}}
}

// FindByRequestID implements Storage.
func (s *storage) FindByRequestID(ctx context.Context, a Address, requestID string) (string, error) {
	return s.legacy.FindByRequestID(ctx, a, requestID)
}

// GenerateFilePath implements Storage.
func (s *storage) GenerateFilePath(ctx context.Context, a Address, timestamp TimeInUTC, reqID string) string {
	return s.legacy.GenerateFilePath(ctx, a, timestamp, reqID)
}

// Latest implements Storage.
func (s *storage) Latest(ctx context.Context, a Address, itemLimit int) []string {
	return s.legacy.Latest(ctx, a, itemLimit)
}

// LatestAfter implements Storage.
func (s *storage) LatestAfter(ctx context.Context, a Address, cutoff TimeInUTC) (string, error) {
	return s.legacy.LatestAfter(ctx, a, cutoff)
}

// RemoveOld implements Storage.
func (s *storage) RemoveOld(ctx context.Context, a Address, retentionDays int) error {
	return s.legacy.RemoveOld(ctx, a, retentionDays)
}

// Rename implements Storage.
func (s *storage) Rename(ctx context.Context, o Address, n Address) error {
	return s.legacy.Rename(ctx, o, n)
}
