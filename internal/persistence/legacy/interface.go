package legacy

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/models"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type HistoryStore interface {
	Open(ctx context.Context, key string, timestamp time.Time, requestID string) error
	Write(ctx context.Context, status models.DAGRunStatus) error
	Close(ctx context.Context) error
	Update(ctx context.Context, key, requestID string, status models.DAGRunStatus) error
	ReadStatusRecent(ctx context.Context, key string, itemLimit int) []StatusFile
	ReadStatusToday(ctx context.Context, key string) (*models.DAGRunStatus, error)
	FindByRequestID(ctx context.Context, key string, requestID string) (*StatusFile, error)
	RemoveAll(ctx context.Context, key string) error
	RemoveOld(ctx context.Context, key string, retentionDays int) error
	Rename(ctx context.Context, oldKey, newKey string) error
}

type StatusFile struct {
	File   string
	Status models.DAGRunStatus
}
