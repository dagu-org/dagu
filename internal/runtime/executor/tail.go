package executor

import (
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
)

// defaultStderrTailLimit is the fallback maximum number of bytes
// to retain from recent stderr output if no override is provided.
const defaultStderrTailLimit = 1024

// TailWriter forwards to an underlying writer and keeps a rolling
// tail of recent output up to `max` bytes. Safe for concurrent use.
type TailWriter struct {
	mu         sync.Mutex
	underlying io.Writer // may be nil; defaults to os.Stderr
	max        int       // maximum bytes to retain in buf
	buf        []byte    // rolling buffer of recent output (raw bytes)
	encoding   string    // character encoding for decoding (e.g., "utf-8", "shift_jis", "euc-jp")
}

// NewTailWriter creates a tailWriter that keeps a rolling buffer
// of recent output with a maximum size of `max` bytes. If max <= 0,
// it falls back to defaultStderrTailLimit.
// NewTailWriter creates a TailWriter that forwards writes to the provided writer and retains a rolling tail of recent bytes.
// If out is nil it defaults to os.Stderr. If max is less than or equal to zero it uses defaultStderrTailLimit.
func NewTailWriter(out io.Writer, max int) *TailWriter {
	if out == nil {
		out = os.Stderr
	}
	if max <= 0 {
		max = defaultStderrTailLimit
	}
	return &TailWriter{underlying: out, max: max}
}

// NewTailWriterWithEncoding creates a TailWriter with character encoding support.
// The encoding parameter specifies the character encoding of the output
// NewTailWriterWithEncoding creates a TailWriter that forwards writes to the given writer,
// retains a rolling buffer limited to max bytes, and sets the character encoding used when
// decoding the buffer returned by Tail().
// If out is nil it defaults to os.Stderr, if max <= 0 the package default limit is used,
// and if encoding is empty UTF-8 is assumed.
func NewTailWriterWithEncoding(out io.Writer, max int, encoding string) *TailWriter {
	tw := NewTailWriter(out, max)
	tw.encoding = encoding
	return tw
}

func (t *TailWriter) Write(p []byte) (int, error) {
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
		t.buf = append(t.buf, p...)
		if len(t.buf) > t.max {
			// Keep only the last t.max bytes
			t.buf = t.buf[len(t.buf)-t.max:]
		}
	}
	t.mu.Unlock()

	return n, err
}

// Tail returns the rolling tail buffer (up to max bytes) as a decoded string.
// If an encoding was specified during creation, the buffer is decoded from
// that encoding to UTF-8. Otherwise, the raw bytes are returned as a string.
func (t *TailWriter) Tail() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return fileutil.DecodeString(t.encoding, t.buf)
}