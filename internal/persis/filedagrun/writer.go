// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedagrun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/core/exec"
)

var (
	ErrWriterNotOpen = errors.New("writer is not open")
)

type Writer struct {
	target     string
	writer     *bufio.Writer
	file       *os.File
	mu         sync.Mutex
	bufferSize int
}

// WriterOption defines functional options for configuring a Writer.
type WriterOption func(*Writer)

// NewWriter creates a new Writer instance for the specified target file path.
func NewWriter(target string, opts ...WriterOption) *Writer {
	w := &Writer{
		target:     target,
		bufferSize: 4096,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Open prepares the writer for writing by creating necessary directories
// and opening the target file.
func (w *Writer) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isOpenLocked() {
		return nil
	}

	dir := filepath.Dir(w.target)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := fileutil.OpenOrCreateFile(w.target)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.target, err)
	}

	w.file = file
	w.writer = bufio.NewWriterSize(file, w.bufferSize)
	return nil
}

// Write serializes the status to JSON and appends it to the file.
// It automatically flushes data to ensure durability.
func (w *Writer) Write(ctx context.Context, st exec.DAGRunStatus) error {
	if err := w.write(st); err != nil {
		logger.Error(ctx, "Failed to write status", tag.Error(err))
		return err
	}

	return nil
}

func (w *Writer) write(st exec.DAGRunStatus) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isOpenLocked() {
		return ErrWriterNotOpen
	}

	jsonBytes, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if _, err := w.writer.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	if err := w.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush data: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync data: %w", err)
	}

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// It's safe to call close multiple times.
func (w *Writer) Close(ctx context.Context) error {
	if err := w.close(); err != nil {
		logger.Error(ctx, "Failed to close writer", tag.Error(err))
		return err
	}

	return nil
}

// close flushes any buffered data and closes the underlying file.
func (w *Writer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isOpenLocked() {
		return nil
	}

	var errs []error

	if err := w.writer.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("flush error: %w", err))
	}

	if err := w.file.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("sync error: %w", err))
	}

	if err := w.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close error: %w", err))
	}

	w.file = nil
	w.writer = nil

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// IsOpen returns true if the writer is currently open.
func (w *Writer) IsOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.isOpenLocked()
}

func (w *Writer) isOpenLocked() bool {
	return w.file != nil && w.writer != nil
}
