package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/masking"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

type OutputCoordinator struct {
	mu sync.Mutex

	stdoutFileName string
	stdoutFile     *os.File
	stdoutWriter   io.Writer

	stderrFileName string
	stderrFile     *os.File
	stderrWriter   io.Writer

	stdoutRedirectFile   *os.File
	stdoutRedirectWriter io.Writer
	StderrRedirectFile   *os.File
	stderrRedirectWriter io.Writer

	// Output capture with size limits to prevent OOM
	outputWriter   *os.File
	outputReader   *os.File
	outputData     string
	outputCaptured bool
	maxOutputSize  int64          // Max output size in bytes
	outputCapture  *outputCapture // Concurrent output capture handler

	// Masker for environment variable masking
	masker *masking.Masker
}

func (oc *OutputCoordinator) StdoutFile() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	return oc.stdoutFileName
}

func (oc *OutputCoordinator) lock() {
	oc.mu.Lock()
}

func (oc *OutputCoordinator) unlock() {
	oc.mu.Unlock()
}

// setupMasker creates a masker for environment variable masking.
func (oc *OutputCoordinator) setupMasker(ctx context.Context, _ NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// Get secrets from DAGContext
	dagCtx := execution.GetDAGContext(ctx)

	// Convert secret envs map to []string format for masker
	var secretEnvs []string
	for k, v := range dagCtx.SecretEnvs {
		secretEnvs = append(secretEnvs, k+"="+v)
	}

	// Create masker if there are secrets to mask
	if len(secretEnvs) > 0 {
		oc.masker = masking.NewMasker(masking.SourcedEnvVars{
			Secrets: secretEnvs,
		})
	} else {
		oc.masker = nil
	}

	return nil
}

func (oc *OutputCoordinator) setup(ctx context.Context, data NodeData) error {
	if err := oc.setupMasker(ctx, data); err != nil {
		return fmt.Errorf("failed to setup masker: %w", err)
	}
	if err := oc.setupWriters(ctx, data); err != nil {
		return err
	}
	if err := oc.setupStdoutRedirect(ctx, data); err != nil {
		return err
	}
	return oc.setupStderrRedirect(ctx, data)
}

func (oc *OutputCoordinator) setupExecutorIO(ctx context.Context, cmd executor.Executor, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var stdout io.Writer = os.Stdout
	if oc.stdoutWriter != nil {
		stdout = oc.stdoutWriter
	}

	// Output to both log and stdout
	if oc.stdoutRedirectWriter != nil {
		stdout = newFlushableMultiWriter(oc.stdoutWriter, oc.stdoutRedirectWriter)
	}

	// Setup output capture only if not already set up
	if data.Step.Output != "" && oc.outputReader == nil {
		var err error
		if oc.outputReader, oc.outputWriter, err = os.Pipe(); err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		logger.Debug(ctx, "Created new output pipes",
			tag.Step(data.Step.Name),
			tag.OutputVar(data.Step.Output),
		)
		// Reset the captured flag to allow new output capture for retry
		oc.outputCaptured = false

		// Get max output size from DAG configuration, default to 1MB
		oc.maxOutputSize = 1024 * 1024 // 1MB default
		if env := execution.GetDAGContext(ctx); env.DAG != nil && env.DAG.MaxOutputSize > 0 {
			oc.maxOutputSize = int64(env.DAG.MaxOutputSize)
		}

		// Reset the output data to empty
		oc.outputData = ""

		// Start concurrent reading to prevent deadlock
		oc.outputCapture = newOutputCapture(oc.maxOutputSize)
		oc.outputCapture.start(ctx, oc.outputReader)
	}

	if oc.outputWriter != nil {
		stdout = newFlushableMultiWriter(stdout, oc.outputWriter)
	}

	cmd.SetStdout(stdout)

	// If stdoutRedirectWriter is set, we write to it as well
	var stderr io.Writer = os.Stderr
	if oc.stderrWriter != nil {
		stderr = oc.stderrWriter
	}
	if oc.stderrRedirectWriter != nil {
		stderr = newFlushableMultiWriter(oc.stderrWriter, oc.stderrRedirectWriter)
	}
	cmd.SetStderr(stderr)

	return nil
}

func (oc *OutputCoordinator) flushWriters() error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error
	for _, w := range []io.Writer{oc.stdoutWriter, oc.stderrWriter, oc.stdoutRedirectWriter, oc.stderrRedirectWriter} {
		if w == nil {
			continue
		}
		switch v := w.(type) {
		case interface{ Flush() error }:
			if err := v.Flush(); err != nil {
				lastErr = err
			}
		case interface{ Sync() error }:
			if err := v.Sync(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (oc *OutputCoordinator) closeResources() error {
	// First flush all writers
	_ = oc.flushWriters()

	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error

	// Close the output writer first to signal EOF to any readers
	if oc.outputWriter != nil {
		_ = oc.outputWriter.Close()
		oc.outputWriter = nil
	}

	// Wait for concurrent capture to finish if it's running
	if oc.outputCapture != nil && !oc.outputCaptured {
		if _, err := oc.outputCapture.wait(); err != nil {
			lastErr = err
		}
	}

	for _, f := range []*os.File{oc.stdoutFile, oc.stderrFile, oc.stdoutRedirectFile, oc.StderrRedirectFile, oc.outputReader} {
		if f != nil {
			if err := f.Sync(); err != nil {
				lastErr = err
			}
			_ = f.Close()
		}
	}
	return lastErr
}

func (oc *OutputCoordinator) setupStdoutRedirect(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stdout == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stdout, data)
	if err != nil {
		return fmt.Errorf("failed to setup stdout file: %w", err)
	}

	oc.stdoutRedirectFile = file
	// Wrap with MaskingWriter if masker is available
	var writer io.Writer = oc.stdoutRedirectFile
	if oc.masker != nil {
		writer = masking.NewMaskingWriter(oc.stdoutRedirectFile, oc.masker)
	}
	oc.stdoutRedirectWriter = newSafeBufferedWriter(writer)

	return nil
}

func (oc *OutputCoordinator) setupStderrRedirect(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stderr == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stderr, data)
	if err != nil {
		return fmt.Errorf("failed to setup stderr file: %w", err)
	}

	oc.StderrRedirectFile = file
	// Wrap with MaskingWriter if masker is available
	var writer io.Writer = oc.StderrRedirectFile
	if oc.masker != nil {
		writer = masking.NewMaskingWriter(oc.StderrRedirectFile, oc.masker)
	}
	oc.stderrRedirectWriter = newSafeBufferedWriter(writer)

	return nil
}

func (oc *OutputCoordinator) setupWriters(_ context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// stdout
	var err error
	oc.stdoutFile, err = fileutil.OpenOrCreateFile(data.State.Stdout)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	// Wrap with MaskingWriter if masker is available
	var stdoutWriter io.Writer = oc.stdoutFile
	if oc.masker != nil {
		stdoutWriter = masking.NewMaskingWriter(oc.stdoutFile, oc.masker)
	}
	oc.stdoutWriter = newSafeBufferedWriter(stdoutWriter)
	oc.stdoutFileName = data.State.Stdout

	// stderr
	oc.stderrFile, err = fileutil.OpenOrCreateFile(data.State.Stderr)
	if err != nil {
		return fmt.Errorf("failed to open stderr file: %w", err)
	}
	// Wrap with MaskingWriter if masker is available
	var stderrWriter io.Writer = oc.stderrFile
	if oc.masker != nil {
		stderrWriter = masking.NewMaskingWriter(oc.stderrFile, oc.masker)
	}
	oc.stderrWriter = newSafeBufferedWriter(stderrWriter)
	oc.stderrFileName = data.State.Stderr

	return nil
}

func (oc *OutputCoordinator) setupFile(ctx context.Context, filePath string, _ NodeData) (*os.File, error) {
	absFilePath := filePath
	if !filepath.IsAbs(absFilePath) {
		dir := GetEnv(ctx).WorkingDir
		absFilePath = filepath.Join(dir, absFilePath)
		absFilePath = filepath.Clean(absFilePath)
	}

	file, err := fileutil.OpenOrCreateFile(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", absFilePath, err)
	}

	return file, nil
}

func (oc *OutputCoordinator) capturedOutput(ctx context.Context) (string, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// Enrich context with output data for all logging in this function
	ctx = logger.WithValues(ctx,
		tag.Output(oc.outputData),
		tag.Length(len(oc.outputData)),
	)

	// Return cached result if already captured
	if oc.outputCaptured {
		logger.Debug(ctx, "Captured output: returning cached")
		return oc.outputData, nil
	}

	// If using concurrent capture, wait for it to complete
	if oc.outputCapture != nil {
		// Close the writer to signal EOF to the reader
		if oc.outputWriter != nil {
			logger.Debug(ctx, "Captured output: closing output writer")
			if err := oc.outputWriter.Close(); err != nil {
				logger.Error(ctx, "Failed to close pipe writer", tag.Error(err))
			}
			oc.outputWriter = nil // Mark as closed
		}

		// Wait for the concurrent reader to finish
		output, err := oc.outputCapture.wait()
		if err != nil {
			return "", err
		}

		// Accumulate output with previous attempts (for retries)
		if oc.outputData != "" && output != "" {
			oc.outputData += "\n" + strings.TrimSpace(output)
		} else if output != "" {
			oc.outputData = strings.TrimSpace(output)
		}

		logger.Debug(ctx, "Captured output",
			tag.Length(len(oc.outputData)),
		)

		// Mark as captured for caching
		oc.outputCaptured = true

		// Close the reader
		if oc.outputReader != nil {
			if err := oc.outputReader.Close(); err != nil {
				logger.Error(ctx, "Failed to close pipe reader", tag.Error(err))
			}
			oc.outputReader = nil
		}

		return oc.outputData, nil
	}

	// Fallback to old behavior if concurrent capture not used (shouldn't happen)
	if oc.outputReader == nil {
		logger.Debug(ctx, "Captured output: no output reader")
		return "", nil
	}

	// Close the writer only if it hasn't been closed already
	if oc.outputWriter != nil {
		logger.Debug(ctx, "Captured output: closing output writer")
		if err := oc.outputWriter.Close(); err != nil {
			logger.Error(ctx, "Failed to close pipe writer", tag.Error(err))
		}
		oc.outputWriter = nil // Mark as closed
	}

	// Use limited reader to prevent OOM
	limitedReader := io.LimitReader(oc.outputReader, oc.maxOutputSize)
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, limitedReader); err != nil {
		return "", fmt.Errorf("io: failed to read output: %w", err)
	}

	output := strings.TrimSpace(buf.String())

	// Check if output was truncated
	if buf.Len() == int(oc.maxOutputSize) {
		logger.Warn(ctx, "Output truncated due to size limit",
			slog.Int64("max-size", oc.maxOutputSize),
		)
		output += "\n[OUTPUT TRUNCATED]"
	}

	// Accumulate output with previous attempts (for retries)
	if oc.outputData != "" && output != "" {
		oc.outputData += "\n" + output
	} else if output != "" {
		oc.outputData = output
	}

	logger.Debug(ctx, "Captured output",
		tag.Length(len(oc.outputData)),
	)

	// Close the reader after reading
	if err := oc.outputReader.Close(); err != nil {
		logger.Error(ctx, "Failed to close pipe reader", tag.Error(err))
	}
	oc.outputReader = nil // Mark as closed

	// Mark as captured for caching
	oc.outputCaptured = true

	return oc.outputData, nil
}

// outputCapture handles concurrent reading from a pipe to avoid deadlocks
// when output exceeds the pipe buffer size (typically 64KB)
type outputCapture struct {
	mu            sync.Mutex
	buffer        bytes.Buffer
	done          chan struct{}
	err           error
	maxSize       int64
	bytesRead     int64
	exceededLimit bool
}

// newOutputCapture creates a new output capture handler
func newOutputCapture(maxSize int64) *outputCapture {
	return &outputCapture{
		done:    make(chan struct{}),
		maxSize: maxSize,
	}
}

// start begins reading from the reader concurrently
func (oc *outputCapture) start(ctx context.Context, reader io.Reader) {
	go func() {
		defer close(oc.done)

		// Enrich context with error tag for all logging in this goroutine
		// Note: error will be set later when needed
		// Read in chunks to detect when we exceed the limit
		buf := make([]byte, 8192) // 8KB chunks
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				oc.mu.Lock()
				if oc.bytesRead+int64(n) > oc.maxSize {
					// Only write up to the limit
					remaining := oc.maxSize - oc.bytesRead
					if remaining > 0 {
						oc.buffer.Write(buf[:remaining])
						oc.bytesRead += remaining
					}
					oc.exceededLimit = true
					oc.mu.Unlock()

					// Continue reading to prevent pipe deadlock but discard the data
					_, _ = io.Copy(io.Discard, reader)
					break
				}
				oc.buffer.Write(buf[:n])
				oc.bytesRead += int64(n)
				oc.mu.Unlock()
			}

			if err != nil {
				if err != io.EOF {
					oc.mu.Lock()
					oc.err = fmt.Errorf("failed to read output: %w", err)
					oc.mu.Unlock()
					logger.Error(ctx, "Failed to capture output", tag.Error(err))
				}
				break
			}
		}
	}()
}

// wait waits for the reading to complete and returns the captured output
func (oc *outputCapture) wait() (string, error) {
	<-oc.done

	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.err != nil {
		return "", oc.err
	}

	if oc.exceededLimit {
		return oc.buffer.String(), fmt.Errorf("output exceeded maximum size limit of %d bytes", oc.maxSize)
	}

	return oc.buffer.String(), nil
}
