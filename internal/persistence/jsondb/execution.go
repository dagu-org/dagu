package jsondb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

type Execution struct {
	baseDir   string
	timestamp time.Time
	requestID string
}

func NewExecution(dir string) (*Execution, error) {
	exec := &Execution{baseDir: dir}
	matches := reExecution.FindStringSubmatch(filepath.Base(dir))
	if len(matches) != 3 {
		return nil, ErrInvalidExecutionDir
	}
	ts, err := parseFileTimestamp(matches[1])
	if err != nil {
		return nil, err
	}
	exec.timestamp = ts
	exec.requestID = matches[2]
	return exec, nil
}

func (e Execution) CreateRecord(_ context.Context, timestamp TimeInUTC, cache *filecache.Cache[*persistence.Status]) (*Record, error) {
	dirName := "attempt_" + timestamp.Format(dateTimeFormatUTC)
	dir := filepath.Join(e.baseDir, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create attempt directory: %w", err)
	}
	return NewRecord(filepath.Join(dir, "status.json"), cache), nil
}

func (e Execution) LatestRecord(_ context.Context, cache *filecache.Cache[*persistence.Status]) (*Record, error) {
	attempts, err := listDirsSorted(e.baseDir, true, reAttempt)
	if err != nil {
		return nil, err
	}
	// Return the first valid attempt
	for _, attempt := range attempts {
		record := NewRecord(filepath.Join(e.baseDir, attempt, "status.json"), cache)
		if record.Exists() {
			return record, nil
		}
	}
	return nil, persistence.ErrNoStatusData
}

func (e Execution) LastUpdated(ctx context.Context) (time.Time, error) {
	latestRecord, err := e.LatestRecord(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	return latestRecord.ModTime()
}

func (e Execution) Remove() error {
	return os.RemoveAll(e.baseDir)
}

var reExecution = regexp.MustCompile(`^exec_(\d{8}_\d{6}_\d{3}Z)_(.*)$`)
var reAttempt = regexp.MustCompile(`^attempt_(\d{8}_\d{6}_\d{3}Z)$`)

func parseFileTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(dateTimeFormatUTC, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse UTC timestamp %s: %w", s, err)
	}
	return t, nil
}
