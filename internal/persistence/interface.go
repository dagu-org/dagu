package persistence

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"time"
)

type HistoryStore interface {
	Open(dagFile string, t time.Time, requestId string) error
	Write(st *model.Status) error
	Close() error
	Update(dagFile, requestId string, st *model.Status) error
	ReadStatusHist(dagFile string, n int) []*model.StatusFile
	ReadStatusToday(dagFile string) (*model.Status, error)
	FindByRequestId(dagFile string, requestId string) (*model.StatusFile, error)
	RemoveAll(dagFile string) error
	RemoveOld(dagFile string, retentionDays int) error
	Rename(oldDAGFile, newDAGFile string) error
}

var (
	ErrRequestIdNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)
