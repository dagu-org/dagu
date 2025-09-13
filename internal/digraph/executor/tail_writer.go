package executor

import (
    "io"
    "os"
    "strings"
    "sync"
)

// tailWriter forwards to an underlying writer and keeps only
// the last completed line written. Safe for concurrent use.
type tailWriter struct {
    mu         sync.Mutex
    underlying io.Writer // may be nil; defaults to os.Stderr
    lastLine   string    // last completed line (without newline)
    partial    string    // carry-over for incomplete line fragments
}

// newTailWriter creates a tailWriter that keeps only the last line.
// If out is nil, it defaults to os.Stderr to preserve exec's behavior.
func newTailWriter(out io.Writer) *tailWriter {
    if out == nil {
        out = os.Stderr
    }
    return &tailWriter{underlying: out}
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

    // Update last completed line
    t.mu.Lock()
    data := t.partial + string(p)
    parts := strings.Split(data, "\n")
    // All except last are complete lines
    complete := parts[:len(parts)-1]
    t.partial = parts[len(parts)-1]
    if len(complete) > 0 {
        t.lastLine = complete[len(complete)-1]
    }
    t.mu.Unlock()

    return n, err
}

// Tail returns the last completed line. If none yet, returns empty string.
func (t *tailWriter) Tail() string {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.lastLine
}
