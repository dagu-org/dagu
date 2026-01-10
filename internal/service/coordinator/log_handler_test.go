package coordinator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
)

func TestStreamTypeToExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		streamType coordinatorv1.LogStreamType
		expected   string
	}{
		{
			name:       "STDOUT",
			streamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
			expected:   "stdout.log",
		},
		{
			name:       "STDERR",
			streamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR,
			expected:   "stderr.log",
		},
		{
			name:       "UNSPECIFIED",
			streamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_UNSPECIFIED,
			expected:   "log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := streamTypeToExtension(tt.streamType)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestLogHandler_StreamKey(t *testing.T) {
	t.Parallel()

	h := newLogHandler("/tmp/logs")

	t.Run("UniqueKeyGeneration", func(t *testing.T) {
		t.Parallel()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		key := h.streamKey(chunk)
		require.Equal(t, "test-dag/run-123/attempt-1/step1/LOG_STREAM_TYPE_STDOUT", key)
	})

	t.Run("DifferentStreamTypesProduceDifferentKeys", func(t *testing.T) {
		t.Parallel()

		chunkStdout := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		chunkStderr := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR,
		}

		keyStdout := h.streamKey(chunkStdout)
		keyStderr := h.streamKey(chunkStderr)

		require.NotEqual(t, keyStdout, keyStderr)
	})
}

func TestLogHandler_LogFilePath(t *testing.T) {
	t.Parallel()

	logDir := "/var/logs"
	h := newLogHandler(logDir)

	t.Run("BasicPathGeneration", func(t *testing.T) {
		t.Parallel()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		path := h.logFilePath(chunk)
		expected := filepath.Join(logDir, "test-dag", "run-123", "attempt-1", "step1.stdout.log")
		require.Equal(t, expected, path)
	})

	t.Run("SubDAGPathUsesRootDAG", func(t *testing.T) {
		t.Parallel()

		chunk := &coordinatorv1.LogChunk{
			DagName:        "sub-dag",
			DagRunId:       "sub-run-456",
			AttemptId:      "attempt-1",
			StepName:       "step1",
			StreamType:     coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
			RootDagRunName: "root-dag",
			RootDagRunId:   "root-run-123",
		}

		path := h.logFilePath(chunk)
		// Should use root DAG's directory
		expected := filepath.Join(logDir, "root-dag", "root-run-123", "attempt-1", "step1.stdout.log")
		require.Equal(t, expected, path)
	})

	t.Run("EmptyAttemptIdFallback", func(t *testing.T) {
		t.Parallel()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "", // Empty
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR,
		}

		path := h.logFilePath(chunk)
		// Should fallback to dagRunID
		expected := filepath.Join(logDir, "test-dag", "run-123", "run-123", "step1.stderr.log")
		require.Equal(t, expected, path)
	})

	t.Run("SafeNameSanitization", func(t *testing.T) {
		t.Parallel()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test/dag:with:special",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step/with/slashes",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		path := h.logFilePath(chunk)
		// Path should be sanitized but still contain the components
		require.Contains(t, path, logDir)
		require.NotContains(t, filepath.Base(path), "/")
	})
}

func TestLogHandler_GetOrCreateWriter(t *testing.T) {
	t.Parallel()

	t.Run("CreatesNewWriter", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)
		defer h.Close()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		writer, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)
		require.NotNil(t, writer)
		require.NotNil(t, writer.file)
		require.NotNil(t, writer.writer)
	})

	t.Run("ReturnsExistingWriter", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)
		defer h.Close()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		writer1, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)

		writer2, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)

		// Should be the same writer
		require.Same(t, writer1, writer2)
	})

	t.Run("CreatesDirectoryStructure", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)
		defer h.Close()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "nested-dag",
			DagRunId:   "run-456",
			AttemptId:  "attempt-2",
			StepName:   "deep-step",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR,
		}

		_, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)

		// Verify directory was created
		expectedDir := filepath.Join(logDir, "nested-dag", "run-456", "attempt-2")
		_, err = os.Stat(expectedDir)
		require.NoError(t, err)
	})

	t.Run("DifferentChunksGetDifferentWriters", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)
		defer h.Close()

		chunk1 := &coordinatorv1.LogChunk{
			DagName:    "dag1",
			DagRunId:   "run-1",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		chunk2 := &coordinatorv1.LogChunk{
			DagName:    "dag2",
			DagRunId:   "run-2",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		writer1, err := h.getOrCreateWriter(chunk1)
		require.NoError(t, err)

		writer2, err := h.getOrCreateWriter(chunk2)
		require.NoError(t, err)

		require.NotSame(t, writer1, writer2)
	})
}

func TestLogHandler_CloseWriter(t *testing.T) {
	t.Parallel()

	t.Run("ClosesAndRemovesWriter", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		// Create a writer
		_, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)

		key := h.streamKey(chunk)
		h.writersMu.Lock()
		_, exists := h.writers[key]
		h.writersMu.Unlock()
		require.True(t, exists)

		// Close the writer
		ctx := context.Background()
		h.closeWriter(ctx, chunk)

		// Verify it's removed
		h.writersMu.Lock()
		_, exists = h.writers[key]
		h.writersMu.Unlock()
		require.False(t, exists)
	})

	t.Run("NoOpForNonExistentKey", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)

		chunk := &coordinatorv1.LogChunk{
			DagName:    "nonexistent-dag",
			DagRunId:   "run-999",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		// Should not panic
		ctx := context.Background()
		h.closeWriter(ctx, chunk)
	})
}

func TestLogHandler_Close(t *testing.T) {
	t.Parallel()

	t.Run("ClosesAllWriters", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)

		// Create multiple writers
		chunks := []*coordinatorv1.LogChunk{
			{
				DagName:    "dag1",
				DagRunId:   "run-1",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
			},
			{
				DagName:    "dag2",
				DagRunId:   "run-2",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDERR,
			},
			{
				DagName:    "dag3",
				DagRunId:   "run-3",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
			},
		}

		for _, chunk := range chunks {
			_, err := h.getOrCreateWriter(chunk)
			require.NoError(t, err)
		}

		h.writersMu.Lock()
		require.Len(t, h.writers, 3)
		h.writersMu.Unlock()

		// Close all
		h.Close()

		// Verify all are removed
		h.writersMu.Lock()
		require.Empty(t, h.writers)
		h.writersMu.Unlock()
	})
}

func TestLogWriter_WriteAndFlush(t *testing.T) {
	t.Parallel()

	t.Run("WritesDataToFile", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := newLogHandler(logDir)
		defer h.Close()

		chunk := &coordinatorv1.LogChunk{
			DagName:    "test-dag",
			DagRunId:   "run-123",
			AttemptId:  "attempt-1",
			StepName:   "step1",
			StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
		}

		writer, err := h.getOrCreateWriter(chunk)
		require.NoError(t, err)

		// Write some data
		testData := []byte("Hello, World!\n")
		n, err := writer.writer.Write(testData)
		require.NoError(t, err)
		require.Equal(t, len(testData), n)

		// Flush and close
		err = writer.writer.Flush()
		require.NoError(t, err)

		ctx := context.Background()
		h.closeWriter(ctx, chunk)

		// Verify file contents
		filePath := h.logFilePath(chunk)
		contents, err := os.ReadFile(filePath)
		require.NoError(t, err)
		require.Equal(t, testData, contents)
	})
}

func TestLogHandler_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	logDir := t.TempDir()
	h := newLogHandler(logDir)
	defer h.Close()

	// Launch multiple goroutines accessing the handler concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			chunk := &coordinatorv1.LogChunk{
				DagName:    "test-dag",
				DagRunId:   "run-123",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
			}

			// All goroutines try to get the same writer
			writer, err := h.getOrCreateWriter(chunk)
			if err != nil {
				t.Errorf("goroutine %d: getOrCreateWriter failed: %v", idx, err)
				done <- false
				return
			}

			// Write some data
			_, err = writer.writer.Write([]byte("test\n"))
			if err != nil {
				t.Errorf("goroutine %d: Write failed: %v", idx, err)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
