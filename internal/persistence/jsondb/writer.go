package jsondb

import (
	"bufio"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/dagu-dev/dagu/internal/utils"
)

// Writer is the interface to write status to local file.
type writer struct {
	target  string
	dagFile string
	writer  *bufio.Writer
	file    *os.File
	mu      sync.Mutex
	closed  bool
}

// Open opens the writer.
func (w *writer) open() (err error) {
	_ = os.MkdirAll(path.Dir(w.target), 0755)
	w.file, err = utils.OpenOrCreateFile(w.target)
	if err == nil {
		w.writer = bufio.NewWriter(w.file)
	}
	return
}

// Writer appends the status to the local file.
func (w *writer) write(st *model.Status) error {
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
func (w *writer) close() (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		err = w.writer.Flush()
		utils.LogErr("flush file", err)
		utils.LogErr("file sync", w.file.Sync())
		utils.LogErr("file close", w.file.Close())
		w.closed = true
	}
	return err
}
