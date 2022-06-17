package logger

import (
	"io"
	"log"
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

// TeeLogger is a logger that writes to both stdout and a file.
type TeeLogger struct {
	Filename string
	file     *os.File
}

// Open opens the file and sets the logger to write to both
// stdout and the file.
func (l *TeeLogger) Open() error {
	dir := path.Dir(l.Filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var err error
	l.file, err = utils.OpenOrCreateFile(l.Filename)
	if err != nil {
		return err
	}
	mw := io.MultiWriter(os.Stdout, l.file)
	log.SetOutput(mw)
	return nil
}

// Close closes the file.
func (l *TeeLogger) Close() error {
	var lastErr error = nil
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			lastErr = err
		}
	}
	log.SetOutput(os.Stdout)
	return lastErr
}
