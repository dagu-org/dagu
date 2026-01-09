package coordinator

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// logHandler handles log streaming from workers
type logHandler struct {
	logDir string

	// Active writers: streamKey -> writer
	writers   map[string]*logWriter
	writersMu sync.Mutex
}

// logWriter manages writing to a single log file
type logWriter struct {
	file   *os.File
	writer *bufio.Writer
	path   string
}

// newLogHandler creates a new log handler
func newLogHandler(logDir string) *logHandler {
	return &logHandler{
		logDir:  logDir,
		writers: make(map[string]*logWriter),
	}
}

// handleStream processes the log stream from a worker
func (h *logHandler) handleStream(stream coordinatorv1.CoordinatorService_StreamLogsServer) error {
	var chunksReceived uint64
	var bytesWritten uint64

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// Stream completed - send response
			return stream.SendAndClose(&coordinatorv1.StreamLogsResponse{
				ChunksReceived: chunksReceived,
				BytesWritten:   bytesWritten,
			})
		}
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %w", err)
		}

		chunksReceived++

		// Handle final marker
		if chunk.IsFinal {
			h.closeWriter(chunk)
			continue
		}

		// Skip empty data
		if len(chunk.Data) == 0 {
			continue
		}

		// Get or create writer for this stream
		writer, err := h.getOrCreateWriter(chunk)
		if err != nil {
			return fmt.Errorf("failed to create writer: %w", err)
		}

		// Write the data
		n, err := writer.writer.Write(chunk.Data)
		if err != nil {
			return fmt.Errorf("failed to write data: %w", err)
		}
		bytesWritten += uint64(n)

		// Flush periodically to ensure data is visible
		if bytesWritten%65536 == 0 {
			_ = writer.writer.Flush()
		}
	}
}

// streamKey creates a unique key for identifying a log stream
func (h *logHandler) streamKey(chunk *coordinatorv1.LogChunk) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		chunk.DagName,
		chunk.DagRunId,
		chunk.StepName,
		chunk.StreamType.String(),
	)
}

// getOrCreateWriter returns an existing writer or creates a new one
func (h *logHandler) getOrCreateWriter(chunk *coordinatorv1.LogChunk) (*logWriter, error) {
	key := h.streamKey(chunk)

	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	// Check if writer already exists
	if w, ok := h.writers[key]; ok {
		return w, nil
	}

	// Create the log file path
	logPath := h.logFilePath(chunk)

	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create the file
	file, err := fileutil.OpenOrCreateFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create buffered writer
	w := &logWriter{
		file:   file,
		writer: bufio.NewWriterSize(file, 64*1024), // 64KB buffer
		path:   logPath,
	}

	h.writers[key] = w
	return w, nil
}

// closeWriter closes and removes a writer
func (h *logHandler) closeWriter(chunk *coordinatorv1.LogChunk) {
	key := h.streamKey(chunk)

	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	if w, ok := h.writers[key]; ok {
		_ = w.writer.Flush()
		_ = w.file.Sync()
		_ = w.file.Close()
		delete(h.writers, key)
	}
}

// logFilePath generates the log file path following the existing pattern
func (h *logHandler) logFilePath(chunk *coordinatorv1.LogChunk) string {
	// Use root DAG info if this is a sub-DAG
	dagName := chunk.DagName
	dagRunID := chunk.DagRunId
	if chunk.RootDagRunId != "" {
		// For sub-DAGs, store under root DAG's directory
		dagName = chunk.RootDagRunName
		dagRunID = chunk.RootDagRunId
	}

	// Determine file extension based on stream type
	var ext string
	switch chunk.StreamType {
	case coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT:
		ext = "stdout.log"
	case coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR:
		ext = "stderr.log"
	default:
		ext = "log"
	}

	// Build path: {logDir}/{dagName}/{dagRunID}/{attemptID}/{stepName}.{ext}
	// If no attemptID, use dagRunID directly
	attemptDir := chunk.AttemptId
	if attemptDir == "" {
		attemptDir = dagRunID
	}

	return filepath.Join(
		h.logDir,
		fileutil.SafeName(dagName),
		fileutil.SafeName(dagRunID),
		attemptDir,
		fmt.Sprintf("%s.%s", fileutil.SafeName(chunk.StepName), ext),
	)
}

// Close closes all open writers
func (h *logHandler) Close() {
	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	for _, w := range h.writers {
		_ = w.writer.Flush()
		_ = w.file.Sync()
		_ = w.file.Close()
	}
	h.writers = make(map[string]*logWriter)
}
