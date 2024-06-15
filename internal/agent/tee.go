package agent

import (
	"io"
	"log"
	"os"
)

// teeWriter is a writer that writes logs to the specified writer and stdout.
type teeWriter struct {
	Writer io.Writer
}

func newTeeWriter(w io.Writer) *teeWriter {
	return &teeWriter{Writer: w}
}

// Open sets the log output to the specified writer and stdout.
func (l *teeWriter) Open() error {
	mw := io.MultiWriter(os.Stdout, l.Writer)
	log.SetOutput(mw)
	return nil
}

// Close resets the log output to stdout.
func (l *teeWriter) Close() {
	log.SetOutput(os.Stdout)
}
