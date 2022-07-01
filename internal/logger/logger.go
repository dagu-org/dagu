package logger

import (
	"io"
	"log"
	"os"
)

type TeeLogger struct {
	*os.File
}

func (l *TeeLogger) Open() error {
	mw := io.MultiWriter(os.Stdout, l.File)
	log.SetOutput(mw)
	return nil
}

func (l *TeeLogger) Close() {
	log.SetOutput(os.Stdout)
}
