package database

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
)

type Writer struct {
	Target string
	writer *bufio.Writer
	file   *os.File
	mu     sync.Mutex
	closed bool
}

func (w *Writer) Open() error {
	if w.closed {
		return fmt.Errorf("file was already closed")
	}
	var err error
	os.MkdirAll(path.Dir(w.Target), 0755)
	w.file, err = utils.OpenOrCreateFile(w.Target)
	if err != nil {
		return err
	}
	w.writer = bufio.NewWriter(w.file)
	return nil
}

func (w *Writer) Write(st *models.Status) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil || w.file == nil {
		return fmt.Errorf("file was not opened")
	}
	jsonb, _ := st.ToJson()
	str := strings.ReplaceAll(string(jsonb), "\n", " ")
	str = strings.ReplaceAll(str, "\r", " ")
	_, err := w.writer.WriteString(str + "\n")
	if err != nil {
		return err
	}
	return w.writer.Flush()
}

func (w *Writer) Close() error {
	if !w.closed {
		if err := w.writer.Flush(); err != nil {
			return err
		}
		w.file.Close()
		w.closed = true
	}
	return nil
}
