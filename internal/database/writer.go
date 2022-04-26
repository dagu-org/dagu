package database

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/yohamta/jobctl/internal/models"
	"github.com/yohamta/jobctl/internal/utils"
)

type Writer struct {
	filename string
	writer   *bufio.Writer
	file     *os.File
	mu       sync.Mutex
	closed   bool
}

func (w *Writer) Open() error {
	if w.closed {
		return fmt.Errorf("file was already closed")
	}
	var err error
	w.file, err = utils.OpenOrCreateFile(w.filename)
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

func (w *Writer) Close() {
	if !w.closed {
		w.file.Close()
		w.closed = true
	}
}
