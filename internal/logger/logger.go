package logger

import (
	"io"
	"log"
	"os"
)

type TeeLogger struct {
	Writer io.Writer
}

func (l *TeeLogger) Open() error {
	mw := io.MultiWriter(os.Stdout, l.Writer)
	log.SetOutput(mw)
	return nil
}

func (l *TeeLogger) Close() {
	log.SetOutput(os.Stdout)
}
