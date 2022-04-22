package agent

import (
	"io"
	"jobctl/internal/utils"
	"log"
	"os"
	"path"
)

type teeLogger struct {
	filename string
	file     *os.File
}

func (l *teeLogger) Open() error {
	dir := path.Dir(l.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var err error
	l.file, err = utils.OpenOrCreateFile(l.filename)
	if err != nil {
		return err
	}
	mw := io.MultiWriter(os.Stdout, l.file)
	log.SetOutput(mw)
	return nil
}

func (l *teeLogger) Close() error {
	var lastErr error = nil
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			lastErr = err
		}
	}
	log.SetOutput(os.Stdout)
	return lastErr
}
