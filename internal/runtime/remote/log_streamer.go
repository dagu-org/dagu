package remote

import (
	"context"
	"io"
	"sync"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// LogStreamType represents the type of log stream
type LogStreamType int

const (
	// StreamTypeStdout represents stdout stream
	StreamTypeStdout LogStreamType = iota + 1
	// StreamTypeStderr represents stderr stream
	StreamTypeStderr
)

// LogStreamer streams logs to coordinator via gRPC
type LogStreamer struct {
	client    coordinatorv1.CoordinatorServiceClient
	workerID  string
	dagRunID  string
	dagName   string
	attemptID string
	rootRef   execution.DAGRunRef
}

// NewLogStreamer creates a new LogStreamer
func NewLogStreamer(
	client coordinatorv1.CoordinatorServiceClient,
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

// NewStepWriter creates a writer that streams to coordinator
// streamType: 1 = stdout, 2 = stderr
func (s *LogStreamer) NewStepWriter(ctx context.Context, stepName string, streamType int) io.WriteCloser {
	return &stepLogWriter{
		ctx:        ctx,
		streamer:   s,
		stepName:   stepName,
		streamType: LogStreamType(streamType),
		buffer:     make([]byte, 0, 32*1024), // 32KB buffer
	}
}

// stepLogWriter implements io.WriteCloser for streaming logs
type stepLogWriter struct {
	ctx        context.Context
	streamer   *LogStreamer
	stepName   string
	streamType LogStreamType
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

	w.sequence++
	chunk := &coordinatorv1.LogChunk{
		WorkerId:       w.streamer.workerID,
		DagRunId:       w.streamer.dagRunID,
		DagName:        w.streamer.dagName,
		StepName:       w.stepName,
		StreamType:     toProtoStreamType(w.streamType),
		Data:           w.buffer,
		Sequence:       w.sequence,
		RootDagRunName: w.streamer.rootRef.Name,
		RootDagRunId:   w.streamer.rootRef.ID,
		AttemptId:      w.streamer.attemptID,
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

	// Flush any remaining data
	if err := w.flush(); err != nil {
		logger.Error(w.ctx, "Failed to flush log buffer", tag.Error(err))
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
			AttemptId:      w.streamer.attemptID,
		}
		if err := w.stream.Send(finalChunk); err != nil {
			logger.Error(w.ctx, "Failed to send final log chunk", tag.Error(err))
		}

		// Close and receive response
		if _, err := w.stream.CloseAndRecv(); err != nil {
			logger.Error(w.ctx, "Failed to close log stream", tag.Error(err))
		}
	}

	return nil
}

// toProtoStreamType converts LogStreamType to proto LogStreamType
func toProtoStreamType(t LogStreamType) coordinatorv1.LogStreamType {
	switch t {
	case StreamTypeStdout:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT
	case StreamTypeStderr:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR
	default:
		return coordinatorv1.LogStreamType_LOG_STREAM_TYPE_UNSPECIFIED
	}
}
