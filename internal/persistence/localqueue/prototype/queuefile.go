package prototype

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

// Errors for the queue file
var (
	ErrQueueFileEmpty = fmt.Errorf("queue file is empty")
)

type QueueFile struct {
	// name is the name of the workflow
	name string
	// baseDir is the base directory for the queue files
	baseDir string
	// Prefix is the prefix for the queue files
	prefix string
	// regex is the regex for matching the queue file name
	regex *regexp.Regexp
	// mu is the mutex for synchronizing access to the queue
	mu sync.RWMutex
}

func NewQueueFile(baseDir, name, prefix string) *QueueFile {
	return &QueueFile{
		baseDir: baseDir,
		name:    name,
		prefix:  prefix,
		regex:   regexp.MustCompile(fmt.Sprintf(`^%s(\d{8}_\d{6})_\d{3}Z_(.*)\.json$`, prefix)),
	}
}

type ItemData struct {
	FileName string              `json:"file_name"`
	Workflow digraph.WorkflowRef `json:"workflow"`
	QueuedAt time.Time           `json:"queued_at"`
}

// Push adds a job to the queue
// Since it's a prototype, it just create a json file with the job ID and workflow reference
func (q *QueueFile) Push(ctx context.Context, workflow digraph.WorkflowRef) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if workflow.Name != q.name {
		return fmt.Errorf("workflow name %s does not match queue name %s", workflow.Name, q.name)
	}

	timestamp := models.NewUTC(time.Now())

	// Create the queue file name
	fileName := queueFileName(q.prefix, workflow.WorkflowID, timestamp)

	// Create the full path for the queue file
	fullPath := filepath.Join(q.baseDir, fileName)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(q.baseDir, os.ModePerm); err != nil {
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
		Workflow: workflow,
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

// DeleteByWorkflowID removes jobs from the queue by workflow ID
func (q *QueueFile) DeleteByWorkflowID(ctx context.Context, workflowID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, item := range items {
		if item.Workflow.WorkflowID == workflowID {
			if err := os.Remove(filepath.Join(q.baseDir, item.FileName)); err != nil {
				return fmt.Errorf("failed to remove queue file %s: %w", item.FileName, err)
			}
		}
	}

	return nil
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

// FindByWorkflowID finds a job by its workflow ID
func (q *QueueFile) FindByWorkflowID(ctx context.Context, workflowID string) (*Job, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check if the base directory exists
	if _, err := os.Stat(q.baseDir); os.IsNotExist(err) {
		return nil, ErrQueueFileEmpty
	}

	// List all files in the base directory
	items, err := q.listItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, item := range items {
		if item.Workflow.WorkflowID == workflowID {
			return NewJob(item), nil
		}
	}

	return nil, fmt.Errorf("workflow ID %s not found in queue", workflowID)
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
	pattern := filepath.Join(q.baseDir, q.prefix+"*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", q.baseDir, err)
	}

	var items []ItemData
	for _, file := range files {
		fileName := filepath.Base(file)
		item, err := q.parseQueueFileName(ctx, fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse queue file name %s: %w", file, err)
		}
		items = append(items, item)
	}

	// Sort the items by queued time
	sort.Slice(items, func(i, j int) bool {
		return items[i].QueuedAt.Before(items[j].QueuedAt)
	})

	return items, nil
}

func (q *QueueFile) parseQueueFileName(_ context.Context, fileName string) (ItemData, error) {
	// Extract the workflow ID and timestamp from the file name
	matches := q.regex.FindStringSubmatch(fileName)
	if len(matches) != 3 {
		return ItemData{}, fmt.Errorf("invalid queue file name format: %s", fileName)
	}

	// Parse the timestamp
	timestamp, err := time.Parse(dateTimeFormatUTC, matches[1])
	if err != nil {
		return ItemData{}, fmt.Errorf("failed to parse timestamp from file name %s: %w", fileName, err)
	}

	// Create the ItemData struct
	item := ItemData{
		FileName: fileName,
		Workflow: digraph.WorkflowRef{
			Name:       q.name,
			WorkflowID: matches[2],
		},
		QueuedAt: timestamp,
	}

	return item, nil
}

func queueFileName(prefix, workflowID string, t models.TimeInUTC) string {
	mill := t.UnixMilli()
	timestamp := t.Format(dateTimeFormatUTC) + "_" + fmt.Sprintf("%03d", mill%1000) + "Z"
	return prefix + timestamp + "_" + workflowID + ".json"
}

// dateTimeFormat is the format used for the timestamp in the queue file name
const dateTimeFormatUTC = "20060102_150405"
