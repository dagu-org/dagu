package jsondb

import (
	"context"
)

type Stroage interface {
	// GenerateFilePath generates a file path for the specified time and request ID.
	GenerateFilePath(ts timeInUTC, requestID string) string
	// FindFiles finds all files with the specified criteria.
	FindFiles(ctx context.Context, dagID string, findOpts ...FindOptions) ([]string, error)
	// FindByRequestID finds a file by request ID.
	FindByRequestID(ctx context.Context, dagID, requestID string) (string, error)
}

func NewLegacyStorage(baseDir, dagName string) *legacyStorage {
	return &legacyStorage{}
}

type FindOptions struct {
}

var _ Stroage = (*legacyStorage)(nil)

type legacyStorage struct {
}

// GenerateFilePath generates a file path for the specified time and request ID.
func (l *legacyStorage) GenerateFilePath(ts timeInUTC, requestID string) string {
	panic("not implemented")
}

// FindFiles finds all files with the specified criteria.
func (l *legacyStorage) FindFiles(ctx context.Context, dagID string, findOpts ...FindOptions) ([]string, error) {
	panic("not implemented")
}

// FindByRequestID finds a file by request ID.
func (l *legacyStorage) FindByRequestID(ctx context.Context, dagID, requestID string) (string, error) {
	panic("not implemented")
}
