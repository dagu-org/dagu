package coordinator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// flushThreshold is the number of bytes after which to flush a writer
const flushThreshold = 65536

// logHandler handles log streaming from workers
type logHandler struct {
	logDir string

	// Active writers: streamKey -> writer
	writers   map[string]*logWriter
	writersMu sync.Mutex
}

// logWriter manages writing to a single log file
type logWriter struct {
	file            *os.File
	writer          *bufio.Writer
	path            string
	bytesSinceFlush uint64 // Track bytes written since last flush
}

// close flushes the buffer, syncs to disk, and closes the file.
// Errors are logged but not returned since this is typically called during cleanup.
func (w *logWriter) close(ctx context.Context) {
	if err := w.writer.Flush(); err != nil {
		logger.Warn(ctx, "Failed to flush log writer",
			slog.String("path", w.path),
			slog.String("error", err.Error()))
	}
	if err := w.file.Sync(); err != nil {
		logger.Warn(ctx, "Failed to sync log file",
			slog.String("path", w.path),
			slog.String("error", err.Error()))
	}
	if err := w.file.Close(); err != nil {
		logger.Warn(ctx, "Failed to close log file",
			slog.String("path", w.path),
			slog.String("error", err.Error()))
	}
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
	ctx := stream.Context()
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
			h.closeWriter(ctx, chunk)
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
		if n > 0 {
			bytesWritten += uint64(n) // #nosec G115 -- n is non-negative from successful Write
			writer.bytesSinceFlush += uint64(n)
		}

		// Flush periodically to ensure data is visible
		if writer.bytesSinceFlush >= flushThreshold {
			if err := writer.writer.Flush(); err != nil {
				return fmt.Errorf("failed to flush log buffer for %s: %w", writer.path, err)
			}
			writer.bytesSinceFlush = 0
		}
	}
}

// streamKey creates a unique key for identifying a log stream.
// Includes AttemptId to prevent collisions during retry scenarios.
func (h *logHandler) streamKey(chunk *coordinatorv1.LogChunk) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		chunk.DagName,
		chunk.DagRunId,
		chunk.AttemptId,
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
func (h *logHandler) closeWriter(ctx context.Context, chunk *coordinatorv1.LogChunk) {
	key := h.streamKey(chunk)

	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	if w, ok := h.writers[key]; ok {
		w.close(ctx)
		delete(h.writers, key)
	}
}

// logFilePath generates the log file path following the existing pattern.
// Path format: {logDir}/{dagName}/{dagRunID}/{attemptID}/{stepName}.{ext}
func (h *logHandler) logFilePath(chunk *coordinatorv1.LogChunk) string {
	dagName := chunk.DagName
	dagRunID := chunk.DagRunId

	// For sub-DAGs, store under root DAG's directory
	if chunk.RootDagRunId != "" {
		dagName = chunk.RootDagRunName
		dagRunID = chunk.RootDagRunId
	}

	attemptDir := chunk.AttemptId
	if attemptDir == "" {
		attemptDir = dagRunID
	}

	ext := streamTypeToExtension(chunk.StreamType)
	filename := fmt.Sprintf("%s.%s", fileutil.SafeName(chunk.StepName), ext)

	return filepath.Join(
		h.logDir,
		fileutil.SafeName(dagName),
		fileutil.SafeName(dagRunID),
		fileutil.SafeName(attemptDir),
		filename,
	)
}

// streamTypeToExtension returns the file extension for a given stream type.
func streamTypeToExtension(streamType coordinatorv1.LogStreamType) string {
	switch streamType {
	case coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT:
		return "stdout.log"
	case coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR:
		return "stderr.log"
	case coordinatorv1.LogStreamType_LOG_STREAM_TYPE_UNSPECIFIED:
		return "log"
	}
	return "log"
}

// Close closes all open writers
func (h *logHandler) Close() {
	h.writersMu.Lock()
	defer h.writersMu.Unlock()

	ctx := context.Background()
	for _, w := range h.writers {
		w.close(ctx)
	}
	h.writers = make(map[string]*logWriter)
}
