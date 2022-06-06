package logger

import (
	"io"
	"log"
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

type TeeLogger struct {
	Filename string
	file     *os.File
}

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
