package legacy

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/persis/legacy/model"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type HistoryStore interface {
	Open(ctx context.Context, key string, timestamp time.Time, requestID string) error
	Write(ctx context.Context, status model.Status) error
	Close(ctx context.Context) error
	Update(ctx context.Context, key, requestID string, status model.Status) error
	ReadStatusRecent(ctx context.Context, key string, itemLimit int) []model.StatusFile
	ReadStatusToday(ctx context.Context, key string) (*model.Status, error)
	FindByRequestID(ctx context.Context, key string, requestID string) (*model.StatusFile, error)
	RemoveAll(ctx context.Context, key string) error
	RemoveOld(ctx context.Context, key string, retentionDays int) error
	Rename(ctx context.Context, oldKey, newKey string) error
}
