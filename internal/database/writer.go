package database

import (
	"bufio"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
)

// Writer is the interface to write status to local file.
type Writer struct {
	Target string
	writer *bufio.Writer
	file   *os.File
	mu     sync.Mutex
	closed bool
}

// Open opens the writer.
func (w *Writer) Open() (err error) {
	os.MkdirAll(path.Dir(w.Target), 0755)
	w.file, err = utils.OpenOrCreateFile(w.Target)
	if err == nil {
		w.writer = bufio.NewWriter(w.file)
	}
	return
}

// Writer appends the status to the local file.
func (w *Writer) Write(st *models.Status) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	jsonb, _ := st.ToJson()
	str := strings.ReplaceAll(string(jsonb), "\n", " ")
	str = strings.ReplaceAll(str, "\r", " ")
	_, err := w.writer.WriteString(str + "\n")
	utils.LogErr("write status", err)
	return w.writer.Flush()
}

// Close closes the writer.
func (w *Writer) Close() (err error) {
	if !w.closed {
		err = w.writer.Flush()
		utils.LogErr("flush file", err)
		utils.LogErr("file sync", w.file.Sync())
		utils.LogErr("file close", w.file.Close())
		w.closed = true
	}
	return err
}
