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

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
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
		match:   regexp.MustCompile(fmt.Sprintf(`^%s%s(\d{8}_\d{6})_(\d{9})Z_(.*)\.json$`, itemPrefix, prefix)),
	}
}

type queuedItem struct {
	file string
	data ItemData
}

// ItemData represents the data stored in the queue file
type ItemData struct {
	FileName string              `json:"fileName"`
	DAGRun   execution.DAGRunRef `json:"dagRun"`
	QueuedAt time.Time           `json:"queuedAt"`
}

// Push adds a job to the queue
// Since it's a prototype, it just create a json file with the job ID and dag-run reference
func (q *QueueFile) Push(ctx context.Context, dagRun execution.DAGRunRef) error {
	ctx = logger.WithValues(ctx,
		tag.Queue(filepath.Base(q.baseDir)),
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
	)
	q.mu.Lock()
	defer q.mu.Unlock()

	timestamp := execution.NewUTC(time.Now())

	// Create the queue file name
	fileName := queueFileName(q.prefix, dagRun.ID, timestamp)

	// Create the full path for the queue file
	fullPath := filepath.Join(q.baseDir, fileName)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(q.baseDir, 0750); err != nil { // nolint: gosec
		logger.Error(ctx, "Failed to create queue directory",
			tag.Dir(q.baseDir),
			tag.Error(err),
		)
		return fmt.Errorf("failed to create directory %s: %w", q.baseDir, err)
	}

	// Create the queue file in temporary directory
	tmpFile, err := os.CreateTemp(q.baseDir, "queue_*")
	if err != nil {
		logger.Error(ctx, "Failed to create temporary queue file",
			tag.Dir(q.baseDir),
			tag.Error(err),
		)
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
		logger.Error(ctx, "Failed to marshal queue item", tag.Error(err))
		return fmt.Errorf("failed to marshal item data: %w", err)
	}

	// Write the data to the file
	if _, err := tmpFile.Write(data); err != nil {
		logger.Error(ctx, "Failed to write queue item", tag.Error(err))
		return fmt.Errorf("failed to write data to queue file %s: %w", fullPath, err)
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		logger.Error(ctx, "Failed to close temporary queue file",
			tag.File(tmpFile.Name()),
			tag.Error(err),
		)
		return fmt.Errorf("failed to close temporary file %s: %w", tmpFile.Name(), err)
	}

	// Rename the temporary file to the final name (this is atomic)
	if err := os.Rename(tmpFile.Name(), fullPath); err != nil {
		logger.Error(ctx, "Failed to rename queue file",
			tag.File(tmpFile.Name()),
			tag.Error(err),
		)
		return fmt.Errorf("failed to rename temporary file %s to %s: %w", tmpFile.Name(), fullPath, err)
	}

	logger.Debug(ctx, "Queued item file written")
	return nil
}

// PopByDAGRunID removes jobs from the queue by dag-run ID
func (q *QueueFile) PopByDAGRunID(ctx context.Context, dagRun execution.DAGRunRef) ([]execution.QueuedItemData, error) {
	ctx = logger.WithValues(ctx,
		tag.Queue(filepath.Base(q.baseDir)),
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
	)
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

	var removedJobs []execution.QueuedItemData
	for _, item := range items {
		job := NewQueuedFile(item.file)
		data, err := job.Data()
		if err != nil {
			logger.Error(ctx, "Failed to get job data",
				tag.File(item.data.FileName),
				tag.Error(err),
			)
			continue
		}
		if data.Name == dagRun.Name && data.ID == dagRun.ID {
			unwrapped, err := job.ExtractJob()
			if err != nil {
				logger.Error(ctx, "Failed to unwrap job",
					tag.File(item.data.FileName),
					tag.Error(err),
				)
				continue
			}

			if err := os.Remove(item.file); err != nil {
				// Log the error but continue processing other items
				logger.Warn(ctx, "Failed to remove queue file",
					tag.File(item.data.FileName),
					tag.Error(err),
				)
			} else {
				// Add the job to the removed jobs list
				removedJobs = append(removedJobs, unwrapped)
			}
		}
	}

	return removedJobs, nil
}

func (q *QueueFile) List(ctx context.Context) ([]*QueuedFile, error) {
	ctx = logger.WithValues(ctx, tag.Queue(filepath.Base(q.baseDir)))
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

	var jobs []*QueuedFile
	for _, item := range items {
		jobs = append(jobs, NewQueuedFile(item.file))
	}

	logger.Debug(ctx, "Listed queue file items", tag.Count(len(jobs)))
	return jobs, nil
}

func (q *QueueFile) Pop(ctx context.Context) (execution.QueuedItemData, error) {
	ctx = logger.WithValues(ctx, tag.Queue(filepath.Base(q.baseDir)))
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

	unwrapped, err := NewQueuedFile(item.file).ExtractJob()
	if err != nil {
		logger.Error(ctx, "Failed to unwrap job",
			tag.File(item.data.FileName),
			tag.Error(err),
		)
		return nil, fmt.Errorf("failed to unwrap job from file %s: %w", item.data.FileName, err)
	}

	// Delete the file
	// Currently, we don't need the content of the file, so we just remove it

	if err := os.Remove(filepath.Join(q.baseDir, item.data.FileName)); err != nil {
		logger.Error(ctx, "Failed to remove queue file",
			tag.File(item.data.FileName),
			tag.Error(err),
		)
		return nil, fmt.Errorf("failed to remove queue file %s: %w", item.data.FileName, err)
	}

	logger.Debug(ctx, "Popped queue file item",
		tag.RunID(item.data.DAGRun.ID),
		tag.DAG(item.data.DAGRun.Name),
	)
	// Return the item data
	return unwrapped, nil
}

// FindByDAGRunID finds a job by its dag-run ID without removing it from the queue.
func (q *QueueFile) FindByDAGRunID(ctx context.Context, dagRunID string) (*QueuedFile, error) {
	ctx = logger.WithValues(ctx, tag.Queue(filepath.Base(q.baseDir)))
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
		if item.data.DAGRun.ID == dagRunID {
			return NewQueuedFile(item.file), nil
		}
	}

	logger.Debug(ctx, "Dag-run not found in queue file", tag.RunID(dagRunID))
	return nil, ErrQueueFileItemNotFound
}

// Len returns the number of items in the queue
func (q *QueueFile) Len(ctx context.Context) (int, error) {
	ctx = logger.WithValues(ctx, tag.Queue(filepath.Base(q.baseDir)))
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

	logger.Debug(ctx, "Queue file length computed", tag.Count(len(items)))
	return len(items), nil
}

// listItems lists all items in the queue directory
// It is not thread-safe and should be called with the mutex locked
// to ensure that no other operations are modifying the queue at the same time.
func (q *QueueFile) listItems(ctx context.Context) ([]queuedItem, error) {
	pattern := filepath.Join(q.baseDir, itemPrefix+q.prefix+"*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		logger.Error(ctx, "Failed to glob queue files",
			tag.Dir(q.baseDir),
			tag.Error(err))
		return nil, fmt.Errorf("failed to list files in %s: %w", q.baseDir, err)
	}

	var items []queuedItem
	for _, file := range files {
		fileName := filepath.Base(file)
		// Check the file name matches the expected pattern
		if !q.match.MatchString(fileName) {
			continue
		}
		// Parse the file name to get the dag-run ID and timestamp
		item, err := parseQueueFileName(file, fileName)
		if err != nil {
			logger.Error(ctx, "Failed to parse queue file name",
				tag.File(fileName),
				tag.Error(err))
			continue
		}
		items = append(items, queuedItem{
			file: file,
			data: item,
		})
	}

	// Sort the items by queued time
	sort.Slice(items, func(i, j int) bool {
		return items[i].data.QueuedAt.Before(items[j].data.QueuedAt)
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

	// Parse the nanoseconds
	nanos, err := strconv.Atoi(matches[3])
	if err != nil {
		return ItemData{}, fmt.Errorf("failed to parse nanoseconds from file name %s: %w", fileName, err)
	}
	timestamp = timestamp.Add(time.Duration(nanos) * time.Nanosecond)

	// Create the ItemData struct
	data := ItemData{
		FileName: fileName,
		DAGRun: execution.DAGRunRef{
			Name: filepath.Base(filepath.Dir(path)),
			ID:   matches[4],
		},
		QueuedAt: timestamp,
	}

	return data, nil
}

// parseRegex is the regex used to parse the queue file name
var parseRegex = regexp.MustCompile(`^item_(high|low)_(\d{8}_\d{6})_(\d{9})Z_(.*)\.json$`)

func queueFileName(priority, dagRunID string, t execution.TimeInUTC) string {
	nanos := t.UnixNano()
	timestamp := t.Format(dateTimeFormatUTC) + "_" + fmt.Sprintf("%09d", nanos%1e9) + "Z"
	return itemPrefix + priority + timestamp + "_" + dagRunID + ".json"
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"
