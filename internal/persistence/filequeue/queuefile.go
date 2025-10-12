package filequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Errors for the queue file
var (
	ErrQueueFileEmpty        = fmt.Errorf("queue file is empty")
	ErrQueueFileItemNotFound = fmt.Errorf("queue file item not found")
)

// itemPrefix is the prefix for the queue file name
const itemPrefix = "item_"

// QueueFile is a simple queue implementation using files
// It stores the queued items in JSON files in a specified directory
// The timestamp is in UTC and the milliseconds are added to the filename
// Since this relies on the file system, it is not thread-safe and should be
// accessed by a single process at a time.
type QueueFile struct {
	// baseDir is the base directory for the queue files
	baseDir string
	// prefix is the prefix for the queue files
	// It is used for differentiating between different priorities
	prefix string
	// match is the match for matching the queue file name
	match *regexp.Regexp
	// mu is the mutex for synchronizing access to the queue
	mu sync.RWMutex
}

// NewQueueFile creates a new queue file with the specified base directory and priority
func NewQueueFile(baseDir, prefix string) *QueueFile {
	return &QueueFile{
		baseDir: baseDir,
		prefix:  prefix,
		match:   regexp.MustCompile(fmt.Sprintf(`^%s%s(\d{8}_\d{6})_(\d{3})Z_(.*)\.json$`, itemPrefix, prefix)),
	}
}

// ItemData represents the data stored in the queue file
type ItemData struct {
	FileName string         `json:"fileName"`
	DAGRun   core.DAGRunRef `json:"dagRun"`
	QueuedAt time.Time      `json:"queuedAt"`
}

// Push adds a job to the queue
// Since it's a prototype, it just create a json file with the job ID and dag-run reference
func (q *QueueFile) Push(_ context.Context, dagRun core.DAGRunRef) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	timestamp := models.NewUTC(time.Now())

	// Create the queue file name
	fileName := queueFileName(q.prefix, dagRun.ID, timestamp)

	// Create the full path for the queue file
	fullPath := filepath.Join(q.baseDir, fileName)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(q.baseDir, 0750); err != nil { // nolint: gosec
		return fmt.Errorf("failed to create directory %s: %w", q.baseDir, err)
	}

	// Create the queue file in temporary directory
	tmpFile, err := os.CreateTemp(q.baseDir, "queue_*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", q.baseDir, err)
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	data, err := json.Marshal(ItemData{
		FileName: fileName,
		DAGRun:   dagRun,
		QueuedAt: timestamp.Time,
	})

	if err != nil {
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	// Write the data to the file
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data to queue file %s: %w", fullPath, err)
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file %s: %w", tmpFile.Name(), err)
	}

	// Rename the temporary file to the final name (this is atomic)
	if err := os.Rename(tmpFile.Name(), fullPath); err != nil {
		return fmt.Errorf("failed to rename temporary file %s to %s: %w", tmpFile.Name(), fullPath, err)
	}

	return nil
}

// PopByDAGRunID removes jobs from the queue by dag-run ID
func (q *QueueFile) PopByDAGRunID(ctx context.Context, dagRunID string) ([]*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var removedJobs []*Job
	for _, item := range items {
		if item.DAGRun.ID == dagRunID {
			if err := os.Remove(filepath.Join(q.baseDir, item.FileName)); err != nil {
				// Log the error but continue processing other items
				logger.Warn(ctx, "failed to remove queue file %s: %w", item.FileName, err)
			} else {
				// Add the job to the removed jobs list
				removedJobs = append(removedJobs, NewJob(item))
			}
		}
	}

	return removedJobs, nil
}

func (q *QueueFile) List(ctx context.Context) ([]*Job, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var jobs []*Job
	for _, item := range items {
		jobs = append(jobs, NewJob(item))
	}

	return jobs, nil
}

func (q *QueueFile) Pop(ctx context.Context) (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil, ErrQueueFileEmpty
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Get the first item in the queue
	if len(items) == 0 {
		return nil, ErrQueueFileEmpty
	}

	item := items[0]

	// Delete the file
	// Currently, we don't need the content of the file, so we just remove it

	if err := os.Remove(filepath.Join(q.baseDir, item.FileName)); err != nil {
		return nil, fmt.Errorf("failed to remove queue file %s: %w", item.FileName, err)
	}

	// Return the item data
	return NewJob(item), nil
}

// FindByDAGRunID finds a job by its dag-run ID without removing it from the queue.
func (q *QueueFile) FindByDAGRunID(ctx context.Context, dagRunID string) (*Job, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil, ErrQueueFileItemNotFound
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, item := range items {
		if item.DAGRun.ID == dagRunID {
			return NewJob(item), nil
		}
	}

	return nil, ErrQueueFileItemNotFound
}

// Len returns the number of items in the queue
func (q *QueueFile) Len(ctx context.Context) (int, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return 0, nil
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	return len(items), nil
}

// listItems lists all items in the queue directory
// It is not thread-safe and should be called with the mutex locked
// to ensure that no other operations are modifying the queue at the same time.
func (q *QueueFile) listItems(ctx context.Context) ([]ItemData, error) {
	pattern := filepath.Join(q.baseDir, itemPrefix+q.prefix+"*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", q.baseDir, err)
	}

	var items []ItemData
	for _, file := range files {
		fileName := filepath.Base(file)
		// Check the file name matches the expected pattern
		if !q.match.MatchString(fileName) {
			continue
		}
		// Parse the file name to get the dag-run ID and timestamp
		item, err := parseQueueFileName(file, fileName)
		if err != nil {
			logger.Error(ctx, "failed to parse queue file name %s: %w", fileName, err)
			continue
		}
		items = append(items, item)
	}

	// Sort the items by queued time
	sort.Slice(items, func(i, j int) bool {
		return items[i].QueuedAt.Before(items[j].QueuedAt)
	})

	return items, nil
}

func parseQueueFileName(path, fileName string) (ItemData, error) {
	// Extract the dag-run ID and timestamp from the file name
	matches := parseRegex.FindStringSubmatch(fileName)
	if len(matches) != 5 {
		return ItemData{}, fmt.Errorf("invalid queue file name format: %s", fileName)
	}

	// Parse the timestamp
	timestamp, err := time.Parse(dateTimeFormatUTC, matches[2])
	if err != nil {
		return ItemData{}, fmt.Errorf("failed to parse timestamp from file name %s: %w", fileName, err)
	}

	// Parse the milliseconds
	millis, err := strconv.Atoi(matches[3])
	if err != nil {
		return ItemData{}, fmt.Errorf("failed to parse milliseconds from file name %s: %w", fileName, err)
	}
	timestamp = timestamp.Add(time.Duration(millis) * time.Millisecond)

	// Create the ItemData struct
	item := ItemData{
		FileName: fileName,
		DAGRun: core.DAGRunRef{
			Name: filepath.Base(filepath.Dir(path)),
			ID:   matches[4],
		},
		QueuedAt: timestamp,
	}

	return item, nil
}

// parseRegex is the regex used to parse the queue file name
var parseRegex = regexp.MustCompile(`^item_(high|low)_(\d{8}_\d{6})_(\d{3})Z_(.*)\.json$`)

func queueFileName(priority, dagRunID string, t models.TimeInUTC) string {
	mill := t.UnixMilli()
	timestamp := t.Format(dateTimeFormatUTC) + "_" + fmt.Sprintf("%03d", mill%1000) + "Z"
	return itemPrefix + priority + timestamp + "_" + dagRunID + ".json"
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"
