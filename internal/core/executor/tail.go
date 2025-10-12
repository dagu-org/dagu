package executor

import (
	"io"
	"os"
	"sync"
)

// defaultStderrTailLimit is the fallback maximum number of bytes
// to retain from recent stderr output if no override is provided.
const defaultStderrTailLimit = 1024

// tailWriter forwards to an underlying writer and keeps a rolling
// tail of recent output up to `max` bytes. Safe for concurrent use.
type tailWriter struct {
	mu         sync.Mutex
	underlying io.Writer // may be nil; defaults to os.Stderr
	max        int       // maximum bytes to retain in buf
	buf        string    // rolling buffer of recent output
}

// newTailWriter creates a tailWriter that keeps a rolling buffer
// of recent output with a maximum size of `max` bytes. If max <= 0,
// it falls back to defaultStderrTailLimit.
// If out is nil, it defaults to os.Stderr to preserve exec's behavior.
func newTailWriter(out io.Writer, max int) *tailWriter {
	if out == nil {
		out = os.Stderr
	}
	if max <= 0 {
		max = defaultStderrTailLimit
	}
	return &tailWriter{underlying: out, max: max}
}

func (t *tailWriter) Write(p []byte) (int, error) {
	// Forward to underlying first
	var n int
	var err error
	if t.underlying != nil {
		n, err = t.underlying.Write(p)
	} else {
		n = len(p)
	}

	// Update rolling buffer, keeping only the last `max` bytes
	t.mu.Lock()
	if len(p) > 0 {
		t.buf += string(p)
		if len(t.buf) > t.max {
			// Keep only the last t.max bytes
			t.buf = t.buf[len(t.buf)-t.max:]
		}
	}
	t.mu.Unlock()

	return n, err
}

// Tail returns the rolling tail buffer (up to max bytes).
func (t *tailWriter) Tail() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf
}
