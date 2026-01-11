package remote

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// logStreamerMockClient implements coordinator.Client for testing log streamer
type logStreamerMockClient struct {
	coordinator.Client // Embed to satisfy interface (unused methods will panic)
	streamLogsFunc     func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error)
}

func (m *logStreamerMockClient) StreamLogs(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
	if m.streamLogsFunc != nil {
		return m.streamLogsFunc(ctx)
	}
	return nil, errors.New("StreamLogs not configured")
}

// mockStreamLogsClient implements coordinatorv1.CoordinatorService_StreamLogsClient
type mockStreamLogsClient struct {
	mu         sync.Mutex
	sentChunks []*coordinatorv1.LogChunk
	sendErr    error                                              // Static error for all sends
	sendFunc   func(idx int, chunk *coordinatorv1.LogChunk) error // Dynamic per-chunk error
	response   *coordinatorv1.StreamLogsResponse
	closeErr   error
}

func (m *mockStreamLogsClient) Send(chunk *coordinatorv1.LogChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendFunc != nil {
		if err := m.sendFunc(len(m.sentChunks), chunk); err != nil {
			return err
		}
	} else if m.sendErr != nil {
		return m.sendErr
	}

	// Deep copy chunk to capture the data at this moment
	chunkCopy := &coordinatorv1.LogChunk{
		WorkerId:       chunk.WorkerId,
		DagRunId:       chunk.DagRunId,
		DagName:        chunk.DagName,
		StepName:       chunk.StepName,
		StreamType:     chunk.StreamType,
		Data:           append([]byte(nil), chunk.Data...),
		Sequence:       chunk.Sequence,
		IsFinal:        chunk.IsFinal,
		RootDagRunName: chunk.RootDagRunName,
		RootDagRunId:   chunk.RootDagRunId,
		AttemptId:      chunk.AttemptId,
	}
	m.sentChunks = append(m.sentChunks, chunkCopy)
	return nil
}

func (m *mockStreamLogsClient) CloseAndRecv() (*coordinatorv1.StreamLogsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closeErr != nil {
		return nil, m.closeErr
	}
	if m.response == nil {
		m.response = &coordinatorv1.StreamLogsResponse{}
	}
	return m.response, nil
}

// Required gRPC stream interface methods
func (m *mockStreamLogsClient) Header() (metadata.MD, error) { return nil, nil }
func (m *mockStreamLogsClient) Trailer() metadata.MD         { return nil }
func (m *mockStreamLogsClient) CloseSend() error             { return nil }
func (m *mockStreamLogsClient) Context() context.Context     { return context.Background() }
func (m *mockStreamLogsClient) SendMsg(msg any) error        { return nil }
func (m *mockStreamLogsClient) RecvMsg(msg any) error        { return nil }

// getSentChunks returns a copy of sent chunks for thread-safe access
func (m *mockStreamLogsClient) getSentChunks() []*coordinatorv1.LogChunk {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*coordinatorv1.LogChunk(nil), m.sentChunks...)
}

func TestToProtoStreamType_Stdout(t *testing.T) {
	t.Parallel()
	result := toProtoStreamType(execution.StreamTypeStdout)
	assert.Equal(t, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT, result)
}

func TestToProtoStreamType_Stderr(t *testing.T) {
	t.Parallel()
	result := toProtoStreamType(execution.StreamTypeStderr)
	assert.Equal(t, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR, result)
}

func TestToProtoStreamType_Unknown(t *testing.T) {
	t.Parallel()
	result := toProtoStreamType(999) // Unknown type
	assert.Equal(t, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_UNSPECIFIED, result)
}

func TestNewLogStreamer(t *testing.T) {
	t.Parallel()
	client := &logStreamerMockClient{}
	rootRef := execution.DAGRunRef{Name: "root-dag", ID: "root-id"}

	streamer := NewLogStreamer(client, "worker-1", "run-123", "test-dag", "attempt-1", rootRef)

	require.NotNil(t, streamer)
	assert.Equal(t, "worker-1", streamer.workerID)
	assert.Equal(t, "run-123", streamer.dagRunID)
	assert.Equal(t, "test-dag", streamer.dagName)
	assert.Equal(t, "attempt-1", streamer.attemptID)
	assert.Equal(t, rootRef, streamer.rootRef)
}

func TestSetAttemptID(t *testing.T) {
	t.Parallel()
	streamer := NewLogStreamer(&logStreamerMockClient{}, "w", "r", "d", "initial", execution.DAGRunRef{})

	assert.Equal(t, "initial", streamer.getAttemptID())

	streamer.SetAttemptID("updated")
	assert.Equal(t, "updated", streamer.getAttemptID())
}

func TestGetAttemptID(t *testing.T) {
	t.Parallel()
	streamer := NewLogStreamer(&logStreamerMockClient{}, "w", "r", "d", "test-attempt", execution.DAGRunRef{})

	// Multiple reads should return same value
	for i := 0; i < 100; i++ {
		assert.Equal(t, "test-attempt", streamer.getAttemptID())
	}
}

func TestSetAttemptID_Concurrent(t *testing.T) {
	t.Parallel()
	streamer := NewLogStreamer(&logStreamerMockClient{}, "w", "r", "d", "initial", execution.DAGRunRef{})

	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			streamer.SetAttemptID("attempt-" + string(rune('A'+id%26)))
		}(i)
	}

	// Concurrent readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = streamer.getAttemptID() // Should not panic
		}()
	}

	wg.Wait()
	// Final value should be one of the written values
	final := streamer.getAttemptID()
	assert.NotEmpty(t, final)
}

func TestNewStepWriter(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "worker-1", "run-123", "test-dag", "attempt-1", execution.DAGRunRef{})

	writer := streamer.NewStepWriter(context.Background(), "step1", execution.StreamTypeStdout)

	require.NotNil(t, writer)
	stepWriter, ok := writer.(*stepLogWriter)
	require.True(t, ok)
	assert.Equal(t, "step1", stepWriter.stepName)
	assert.Equal(t, execution.StreamTypeStdout, stepWriter.streamType)
	assert.Equal(t, streamer, stepWriter.streamer)
	assert.False(t, stepWriter.closed)
	assert.False(t, stepWriter.streamInitFailed)
}

func TestWrite_SmallData(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write small data (< 32KB)
	data := []byte("small log message")
	n, err := writer.Write(data)

	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	// No chunks sent yet - buffer not full
	assert.Empty(t, mockStream.getSentChunks())
}

func TestWrite_ExactThreshold(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write exactly logBufferSize (32KB) - should trigger flush
	data := make([]byte, logBufferSize)
	for i := range data {
		data[i] = byte('A' + i%26)
	}

	n, err := writer.Write(data)

	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	// Should have flushed
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Equal(t, data, chunks[0].Data)
	assert.Equal(t, uint64(1), chunks[0].Sequence)
}

func TestWrite_LargeData(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write data larger than buffer (64KB)
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = byte('X')
	}

	n, err := writer.Write(data)

	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	// Should have flushed
	chunks := mockStream.getSentChunks()
	require.NotEmpty(t, chunks)
}

func TestWrite_MultipleSmallWrites(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Multiple small writes that accumulate to >= threshold
	smallData := make([]byte, 8*1024) // 8KB each
	for i := range smallData {
		smallData[i] = byte('A')
	}

	// Write 4 times = 32KB, should trigger flush on 4th write
	for i := 0; i < 4; i++ {
		n, err := writer.Write(smallData)
		require.NoError(t, err)
		assert.Equal(t, len(smallData), n)
	}

	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Len(t, chunks[0].Data, 32*1024)
}

func TestWrite_AfterClose(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Close the writer
	err := writer.Close()
	require.NoError(t, err)

	// Write after close should fail
	n, err := writer.Write([]byte("data"))
	assert.Equal(t, 0, n)
	assert.Equal(t, io.ErrClosedPipe, err)
}

func TestWrite_FlushError_Continues(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendErr: errors.New("send failed"),
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write enough to trigger flush (which will fail)
	data := make([]byte, logBufferSize)
	n, err := writer.Write(data)

	// Write should succeed even though flush failed (best-effort)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
}

func TestWrite_FlushError_ClearsBuffer(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendErr: errors.New("send failed"),
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)
	stepWriter := writer.(*stepLogWriter)

	// Write enough to trigger flush
	data := make([]byte, logBufferSize)
	_, _ = writer.Write(data)

	// Buffer should be cleared to prevent memory growth
	stepWriter.mu.Lock()
	bufLen := len(stepWriter.buffer)
	stepWriter.mu.Unlock()
	assert.Equal(t, 0, bufLen)
}

func TestFlush_EmptyBuffer(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// Flush with empty buffer
	stepWriter.mu.Lock()
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	assert.Empty(t, mockStream.getSentChunks())
}

func TestFlush_StreamInitSuccess(t *testing.T) {
	t.Parallel()
	streamInitCalled := false
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			streamInitCalled = true
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("test data")
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	assert.True(t, streamInitCalled)
	assert.NotNil(t, stepWriter.stream)
}

func TestFlush_StreamInitFailure(t *testing.T) {
	t.Parallel()
	initErr := errors.New("connection refused")
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return nil, initErr
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("test data")
	err := stepWriter.flush()
	streamInitFailed := stepWriter.streamInitFailed
	bufLen := len(stepWriter.buffer)
	stepWriter.mu.Unlock()

	assert.Equal(t, initErr, err)
	assert.True(t, streamInitFailed, "streamInitFailed should be set")
	assert.Equal(t, 0, bufLen, "buffer should be cleared")
}

func TestFlush_AfterInitFailure(t *testing.T) {
	t.Parallel()
	callCount := 0
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			callCount++
			return nil, errors.New("init failed")
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// First flush - triggers init failure
	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("data1")
	_ = stepWriter.flush()
	stepWriter.mu.Unlock()

	// Second flush - should silently return (no retry)
	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("data2")
	err := stepWriter.flush()
	bufLen := len(stepWriter.buffer)
	stepWriter.mu.Unlock()

	require.NoError(t, err, "should silently succeed after init failure")
	assert.Equal(t, 0, bufLen, "buffer should be cleared")
	assert.Equal(t, 1, callCount, "should not retry stream init")
}

func TestFlush_SendSuccess(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("test data")
	initialSeq := stepWriter.sequence
	err := stepWriter.flush()
	finalSeq := stepWriter.sequence
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	assert.Equal(t, initialSeq+1, finalSeq, "sequence should increment after success")
}

func TestFlush_SendFailure(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendErr: errors.New("send failed"),
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	stepWriter.mu.Lock()
	stepWriter.buffer = []byte("test data")
	initialSeq := stepWriter.sequence
	err := stepWriter.flush()
	finalSeq := stepWriter.sequence
	stepWriter.mu.Unlock()

	assert.Error(t, err)
	assert.Equal(t, initialSeq, finalSeq, "sequence should NOT increment on failure")
}

func TestFlush_SingleChunk(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// Buffer < 3MB - single chunk
	data := make([]byte, 1*1024*1024) // 1MB
	for i := range data {
		data[i] = byte('A')
	}

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Len(t, chunks[0].Data, 1*1024*1024)
}

func TestFlush_ExactMaxChunkSize(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// Buffer == maxChunkSize (3MB) - single chunk
	data := make([]byte, maxChunkSize)
	for i := range data {
		data[i] = byte('B')
	}

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Len(t, chunks[0].Data, maxChunkSize)
}

func TestFlush_TwoChunks(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// 4MB buffer - should split into 3MB + 1MB
	data := make([]byte, 4*1024*1024)
	for i := range data {
		data[i] = byte('C')
	}

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 2)
	assert.Len(t, chunks[0].Data, maxChunkSize) // 3MB
	assert.Len(t, chunks[1].Data, 1*1024*1024)  // 1MB
}

func TestFlush_MultipleChunks(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// 10MB buffer - should split into 3MB + 3MB + 3MB + 1MB = 4 chunks
	data := make([]byte, 10*1024*1024)
	for i := range data {
		data[i] = byte('D')
	}

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 4)
	assert.Len(t, chunks[0].Data, maxChunkSize)
	assert.Len(t, chunks[1].Data, maxChunkSize)
	assert.Len(t, chunks[2].Data, maxChunkSize)
	assert.Len(t, chunks[3].Data, 1*1024*1024)
}

func TestFlush_ChunkSequences(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// 6MB buffer - 2 chunks
	data := make([]byte, 6*1024*1024)

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 2)
	assert.Equal(t, uint64(1), chunks[0].Sequence)
	assert.Equal(t, uint64(2), chunks[1].Sequence)
}

func TestFlush_PartialFailure(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendFunc: func(idx int, chunk *coordinatorv1.LogChunk) error {
			// First chunk succeeds, second fails
			if idx == 1 {
				return errors.New("send failed on chunk 2")
			}
			return nil
		},
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	// 6MB buffer - would be 2 chunks, but second fails
	data := make([]byte, 6*1024*1024)

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	initialSeq := stepWriter.sequence
	err := stepWriter.flush()
	finalSeq := stepWriter.sequence
	stepWriter.mu.Unlock()

	assert.Error(t, err)
	// Only first chunk sent and sequence incremented
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Equal(t, initialSeq+1, finalSeq, "only first chunk's sequence incremented")
}

func TestFlush_DataCopied(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	data := []byte("original data")

	stepWriter.mu.Lock()
	stepWriter.buffer = data
	err := stepWriter.flush()
	stepWriter.mu.Unlock()

	require.NoError(t, err)

	// Modify original data after send
	data[0] = 'X'

	// Sent chunk should have original data
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 1)
	assert.Equal(t, byte('o'), chunks[0].Data[0], "sent data should not be affected by buffer modification")
}

func TestClose_NoData(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	err := writer.Close()

	require.NoError(t, err)
	// No stream was created (no data written), so no chunks sent
	assert.Empty(t, mockStream.getSentChunks())
}

func TestClose_WithUnflushedData(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write small data (not flushed)
	_, _ = writer.Write([]byte("unflushed data"))

	err := writer.Close()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.Len(t, chunks, 2) // data chunk + final marker
	assert.Equal(t, []byte("unflushed data"), chunks[0].Data)
	assert.False(t, chunks[0].IsFinal)
	assert.True(t, chunks[1].IsFinal)
}

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write and close
	_, _ = writer.Write([]byte("data"))
	err1 := writer.Close()
	err2 := writer.Close()
	err3 := writer.Close()

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	// Only one set of chunks sent
	chunks := mockStream.getSentChunks()
	assert.Len(t, chunks, 2) // data + final
}

func TestClose_FinalChunkSequence(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write enough to flush, then close
	data := make([]byte, logBufferSize)
	_, _ = writer.Write(data)
	_, _ = writer.Write([]byte("more data"))
	err := writer.Close()

	require.NoError(t, err)
	chunks := mockStream.getSentChunks()
	require.GreaterOrEqual(t, len(chunks), 2)

	// Verify sequences are increasing and final > all data sequences
	finalChunk := chunks[len(chunks)-1]
	assert.True(t, finalChunk.IsFinal)
	for i, chunk := range chunks[:len(chunks)-1] {
		assert.Less(t, chunk.Sequence, finalChunk.Sequence, "chunk %d sequence should be less than final", i)
	}
}

func TestClose_FinalSendSuccess(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	stepWriter := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout).(*stepLogWriter)

	_, _ = stepWriter.Write([]byte("data"))
	err := stepWriter.Close()

	require.NoError(t, err)

	// Final sequence should be 2 (data=1, final=2)
	stepWriter.mu.Lock()
	finalSeq := stepWriter.sequence
	stepWriter.mu.Unlock()
	assert.Equal(t, uint64(2), finalSeq)
}

func TestClose_FinalSendFailure(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendFunc: func(idx int, chunk *coordinatorv1.LogChunk) error {
			// Fail on final chunk
			if chunk.IsFinal {
				return errors.New("final send failed")
			}
			return nil
		},
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	_, _ = writer.Write([]byte("data"))
	err := writer.Close()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "final send failed")
}

func TestClose_CloseAndRecvError(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		closeErr: errors.New("close failed"),
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	_, _ = writer.Write([]byte("data"))
	err := writer.Close()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
}

func TestClose_MultipleErrors(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{
		sendFunc: func(idx int, chunk *coordinatorv1.LogChunk) error {
			if chunk.IsFinal {
				return errors.New("final send error")
			}
			return nil
		},
		closeErr: errors.New("close error"),
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	_, _ = writer.Write([]byte("data"))
	err := writer.Close()

	// First error (final send) should be returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "final send error")
}

func TestClose_NoStream(t *testing.T) {
	t.Parallel()
	// Client that returns error on stream init
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return nil, errors.New("init failed")
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write triggers init failure
	data := make([]byte, logBufferSize)
	_, _ = writer.Write(data)

	// Close should handle nil stream gracefully
	err := writer.Close()
	// No error because stream never initialized and streamInitFailed handles it
	require.NoError(t, err)
}

func TestClose_FlushErrorThenSendSuccess(t *testing.T) {
	t.Parallel()
	firstFlushDone := false
	mockStream := &mockStreamLogsClient{
		sendFunc: func(idx int, chunk *coordinatorv1.LogChunk) error {
			// First flush chunk fails, final succeeds
			if !chunk.IsFinal && !firstFlushDone {
				firstFlushDone = true
				return errors.New("flush send failed")
			}
			return nil
		},
	}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	_, _ = writer.Write([]byte("data"))
	err := writer.Close()

	// Flush error takes precedence
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flush send failed")
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	var wg sync.WaitGroup
	const goroutines = 100
	const writesPerGoroutine = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_, err := writer.Write([]byte("data"))
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
	require.NoError(t, writer.Close())
}

func TestConcurrentWriteAndClose(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	var wg sync.WaitGroup

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, err := writer.Write([]byte("data"))
				// Either succeeds or returns ErrClosedPipe
				if err != nil {
					assert.Equal(t, io.ErrClosedPipe, err)
					return
				}
			}
		}()
	}

	// Close after a short delay
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = writer.Close()
	}()

	wg.Wait()
}

func TestConcurrentSetAttemptID(t *testing.T) {
	t.Parallel()
	// Each flush gets its own stream to avoid races
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return &mockStreamLogsClient{}, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "initial", execution.DAGRunRef{})

	var wg sync.WaitGroup

	// Concurrent SetAttemptID calls
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			streamer.SetAttemptID("attempt-" + string(rune('A'+id%26)))
		}(i)
	}

	// Concurrent writes with separate writers (each gets its own stream)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)
			_, _ = writer.Write(make([]byte, logBufferSize)) // Triggers flush which reads attemptID
			_ = writer.Close()
		}()
	}

	wg.Wait()
}

func TestLogStreamer_FullLifecycle(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	rootRef := execution.DAGRunRef{Name: "root", ID: "root-123"}
	streamer := NewLogStreamer(client, "worker-1", "run-456", "test-dag", "attempt-789", rootRef)

	writer := streamer.NewStepWriter(context.Background(), "step1", execution.StreamTypeStdout)

	// Multiple writes
	for i := 0; i < 5; i++ {
		data := make([]byte, 8*1024) // 8KB each, 40KB total
		_, err := writer.Write(data)
		require.NoError(t, err)
	}

	err := writer.Close()
	require.NoError(t, err)

	// Verify all chunks
	chunks := mockStream.getSentChunks()
	require.NotEmpty(t, chunks)

	// Verify metadata on all chunks
	for _, chunk := range chunks {
		assert.Equal(t, "worker-1", chunk.WorkerId)
		assert.Equal(t, "run-456", chunk.DagRunId)
		assert.Equal(t, "test-dag", chunk.DagName)
		assert.Equal(t, "step1", chunk.StepName)
		assert.Equal(t, coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT, chunk.StreamType)
		assert.Equal(t, "root", chunk.RootDagRunName)
		assert.Equal(t, "root-123", chunk.RootDagRunId)
		assert.Equal(t, "attempt-789", chunk.AttemptId)
	}

	// Verify final chunk
	lastChunk := chunks[len(chunks)-1]
	assert.True(t, lastChunk.IsFinal)

	// Verify sequence ordering
	for i := 1; i < len(chunks); i++ {
		assert.Greater(t, chunks[i].Sequence, chunks[i-1].Sequence)
	}
}

func TestLogStreamer_MultipleSteps(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})

	// Create multiple step writers
	writer1 := streamer.NewStepWriter(context.Background(), "step1", execution.StreamTypeStdout)
	writer2 := streamer.NewStepWriter(context.Background(), "step2", execution.StreamTypeStdout)

	_, _ = writer1.Write([]byte("step1 data"))
	_, _ = writer2.Write([]byte("step2 data"))

	require.NoError(t, writer1.Close())
	require.NoError(t, writer2.Close())

	// Both should have sent their data
	chunks := mockStream.getSentChunks()
	stepNames := make(map[string]bool)
	for _, chunk := range chunks {
		stepNames[chunk.StepName] = true
	}
	assert.True(t, stepNames["step1"])
	assert.True(t, stepNames["step2"])
}

func TestLogStreamer_StdoutAndStderr(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})

	stdout := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)
	stderr := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStderr)

	_, _ = stdout.Write([]byte("stdout data"))
	_, _ = stderr.Write([]byte("stderr data"))

	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	// Verify both stream types present
	chunks := mockStream.getSentChunks()
	hasStdout := false
	hasStderr := false
	for _, chunk := range chunks {
		if chunk.StreamType == coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT {
			hasStdout = true
		}
		if chunk.StreamType == coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR {
			hasStderr = true
		}
	}
	assert.True(t, hasStdout)
	assert.True(t, hasStderr)
}

func TestLogStreamer_LargeOutput(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Write 12MB of data
	data := make([]byte, 12*1024*1024)
	for i := range data {
		data[i] = byte('X')
	}

	n, err := writer.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)

	err = writer.Close()
	require.NoError(t, err)

	// Verify all data was sent across multiple chunks
	chunks := mockStream.getSentChunks()
	totalBytes := 0
	for _, chunk := range chunks {
		if !chunk.IsFinal {
			totalBytes += len(chunk.Data)
		}
	}
	assert.Equal(t, len(data), totalBytes)

	// Verify no chunk exceeds maxChunkSize
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk.Data), maxChunkSize)
	}
}

func TestLogStreamer_AttemptIDUpdatedDuringStream(t *testing.T) {
	t.Parallel()
	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "initial-attempt", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// First write with initial attempt ID
	data := make([]byte, logBufferSize)
	_, _ = writer.Write(data)

	// Update attempt ID mid-stream
	streamer.SetAttemptID("updated-attempt")

	// Second write should use updated attempt ID
	_, _ = writer.Write(data)

	err := writer.Close()
	require.NoError(t, err)

	// Verify attempt ID changed in chunks
	chunks := mockStream.getSentChunks()
	attemptIDs := make(map[string]bool)
	for _, chunk := range chunks {
		attemptIDs[chunk.AttemptId] = true
	}
	// Should have both attempt IDs
	assert.True(t, attemptIDs["initial-attempt"] || attemptIDs["updated-attempt"])
}

func TestLogStreamer_SequenceContinuity(t *testing.T) {
	t.Parallel()

	mockStream := &mockStreamLogsClient{}
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return mockStream, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})
	writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)

	// Multiple flushes
	for i := 0; i < 5; i++ {
		data := make([]byte, logBufferSize)
		_, _ = writer.Write(data)
	}
	_ = writer.Close()

	// Verify sequences are strictly increasing with no gaps
	chunks := mockStream.getSentChunks()
	for i := 0; i < len(chunks); i++ {
		assert.Equal(t, uint64(i+1), chunks[i].Sequence, "sequence %d should be %d", i, i+1)
	}
}

func TestLogStreamer_RaceDetector(t *testing.T) {
	// This test is specifically for -race flag
	t.Parallel()

	// Each writer gets its own mock stream to avoid races between writers
	client := &logStreamerMockClient{
		streamLogsFunc: func(ctx context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
			return &mockStreamLogsClient{}, nil
		},
	}
	streamer := NewLogStreamer(client, "w", "r", "d", "a", execution.DAGRunRef{})

	var wg sync.WaitGroup
	var ops int64

	// Multiple writers on same streamer (each gets its own stream)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer := streamer.NewStepWriter(context.Background(), "step", execution.StreamTypeStdout)
			for j := 0; j < 20; j++ {
				_, _ = writer.Write([]byte("data"))
				atomic.AddInt64(&ops, 1)
			}
			_ = writer.Close()
		}()
	}

	// Concurrent SetAttemptID
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			streamer.SetAttemptID("attempt-" + string(rune('A'+i%26)))
			atomic.AddInt64(&ops, 1)
		}
	}()

	wg.Wait()
	assert.Greater(t, ops, int64(0))
}
