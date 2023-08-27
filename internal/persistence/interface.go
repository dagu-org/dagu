package persistence

import (
	"fmt"
	"github.com/yohamta/dagu/internal/models"
	"time"
)

type HistoryStore interface {
	Open(dagFile string, t time.Time, requestId string) error
	Write(st *models.Status) error
	Close() error
	Update(dagFile, requestId string, st *models.Status) error
	ReadStatusHist(dagFile string, n int) []*models.StatusFile
	ReadStatusToday(dagFile string) (*models.Status, error)
	FindByRequestId(dagFile string, requestId string) (*models.StatusFile, error)
	RemoveAll(dagFile string) error
	RemoveOld(dagFile string, retentionDays int) error
	Rename(oldDAGFile, newDAGFile string) error
}

var (
	ErrRequestIdNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)
