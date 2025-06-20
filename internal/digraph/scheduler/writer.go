package scheduler

import (
	"bufio"
	"io"
)

// flushableMultiWriter creates a MultiWriter that can flush all underlying writers
type flushableMultiWriter struct {
	writers []io.Writer
}

// newFlushableMultiWriter creates a new flushableMultiWriter
func newFlushableMultiWriter(writers ...io.Writer) *flushableMultiWriter {
	return &flushableMultiWriter{writers: writers}
}

// Write writes to all underlying writers
func (fw *flushableMultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range fw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// Flush flushes all underlying writers that support flushing
func (fw *flushableMultiWriter) Flush() error {
	var lastErr error
	for _, w := range fw.writers {
		// Try different flush interfaces
		switch v := w.(type) {
		case *bufio.Writer:
			if err := v.Flush(); err != nil {
				lastErr = err
			}
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