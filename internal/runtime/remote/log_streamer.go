package remote

import (
	"context"
	"io"
	"sync"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// LogStreamer streams logs to coordinator via gRPC
type LogStreamer struct {
	client    coordinator.Client
	workerID  string
	dagRunID  string
	dagName   string
	attemptID string
	rootRef   execution.DAGRunRef
	mu        sync.RWMutex
}

// NewLogStreamer creates a new LogStreamer
func NewLogStreamer(
	client coordinator.Client,
	workerID string,
	dagRunID string,
	dagName string,
	attemptID string,
	rootRef execution.DAGRunRef,
) *LogStreamer {
	return &LogStreamer{
		client:    client,
		workerID:  workerID,
		dagRunID:  dagRunID,
		dagName:   dagName,
		attemptID: attemptID,
		rootRef:   rootRef,
	}
}

// SetAttemptID updates the attemptID after the agent creates the attempt
func (s *LogStreamer) SetAttemptID(attemptID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attemptID = attemptID
}

// getAttemptID returns the current attemptID
func (s *LogStreamer) getAttemptID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.attemptID
}

// NewStepWriter creates a writer that streams to coordinator
// streamType should be execution.StreamTypeStdout or execution.StreamTypeStderr
func (s *LogStreamer) NewStepWriter(ctx context.Context, stepName string, streamType int) io.WriteCloser {
	return &stepLogWriter{
		ctx:        ctx,
		streamer:   s,
		stepName:   stepName,
		streamType: streamType,
		buffer:     make([]byte, 0, 32*1024), // 32KB buffer
	}
}

// stepLogWriter implements io.WriteCloser for streaming logs
type stepLogWriter struct {
	ctx        context.Context
	streamer   *LogStreamer
	stepName   string
	streamType int
	buffer     []byte
	sequence   uint64
	stream     coordinatorv1.CoordinatorService_StreamLogsClient
	mu         sync.Mutex
	closed     bool
}

// Write implements io.Writer
func (w *stepLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, io.ErrClosedPipe
	}

	w.buffer = append(w.buffer, p...)

	// Flush when buffer exceeds threshold
	if len(w.buffer) >= 32*1024 {
		if err := w.flush(); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

// flush sends buffered data to coordinator
func (w *stepLogWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}

	// Initialize stream if needed
	if w.stream == nil {
		var err error
		w.stream, err = w.streamer.client.StreamLogs(w.ctx)
		if err != nil {
			return err
		}
	}

	// Copy buffer to avoid data corruption if Send buffers the message
	dataCopy := make([]byte, len(w.buffer))
	copy(dataCopy, w.buffer)

	w.sequence++
	chunk := &coordinatorv1.LogChunk{
		WorkerId:       w.streamer.workerID,
		DagRunId:       w.streamer.dagRunID,
		DagName:        w.streamer.dagName,
		StepName:       w.stepName,
		StreamType:     toProtoStreamType(w.streamType),
		Data:           dataCopy,
		Sequence:       w.sequence,
		RootDagRunName: w.streamer.rootRef.Name,
		RootDagRunId:   w.streamer.rootRef.ID,
		AttemptId:      w.streamer.getAttemptID(),
	}

	w.buffer = w.buffer[:0]
	return w.stream.Send(chunk)
}

// Close implements io.Closer
func (w *stepLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error

	// Flush any remaining data
	if err := w.flush(); err != nil {
		logger.Error(w.ctx, "Failed to flush log buffer", tag.Error(err))
		firstErr = err
	}

	// Send final marker
	if w.stream != nil {
		finalChunk := &coordinatorv1.LogChunk{
			WorkerId:       w.streamer.workerID,
			DagRunId:       w.streamer.dagRunID,
			DagName:        w.streamer.dagName,
			StepName:       w.stepName,
			StreamType:     toProtoStreamType(w.streamType),
			IsFinal:        true,
			RootDagRunName: w.streamer.rootRef.Name,
			RootDagRunId:   w.streamer.rootRef.ID,
			AttemptId:      w.streamer.getAttemptID(),
		}
		if err := w.stream.Send(finalChunk); err != nil {
			logger.Error(w.ctx, "Failed to send final log chunk", tag.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}

		// Close and receive response
		if _, err := w.stream.CloseAndRecv(); err != nil {
			logger.Error(w.ctx, "Failed to close log stream", tag.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// toProtoStreamType converts streamType int to proto LogStreamType
func toProtoStreamType(streamType int) coordinatorv1.LogStreamType {
	switch streamType {
	case execution.StreamTypeStdout:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT
	case execution.StreamTypeStderr:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR
	default:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_UNSPECIFIED
	}
}
