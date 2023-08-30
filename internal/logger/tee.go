package logger

import (
	"io"
	"log"
	"os"
)

type Tee struct {
	Writer io.Writer
}

func (l *Tee) Open() error {
	mw := io.MultiWriter(os.Stdout, l.Writer)
	log.SetOutput(mw)
	return nil
}

func (l *Tee) Close() {
	log.SetOutput(os.Stdout)
}
