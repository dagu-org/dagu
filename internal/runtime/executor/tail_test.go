package executor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTailWriter_RollingBufferSimple(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tw := NewTailWriter(&buf, 100)

	input := "line1\nline2\n"
	n, err := tw.Write([]byte(input))
	assert.NoError(t, err)
	assert.Equal(t, len(input), n)

	// Underlying receives full content
	assert.Equal(t, input, buf.String())
	// Tail returns recent output (not just the last line)
	assert.Equal(t, input, tw.Tail())
}

func TestTailWriter_AcrossWritesAndLimit(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	limit := 10
	tw := NewTailWriter(&buf, limit)

	_, _ = tw.Write([]byte("one\nlin")) // "one\nlin"
	assert.True(t, len(tw.Tail()) <= limit)
	assert.True(t, bytes.HasSuffix([]byte(tw.Tail()), []byte("one\nlin")))

	_, _ = tw.Write([]byte("e2\npar")) // "one\nline2\npar"
	assert.True(t, len(tw.Tail()) <= limit)
	assert.True(t, bytes.HasSuffix([]byte(tw.Tail()), []byte("e2\npar")))

	_, _ = tw.Write([]byte("tial")) // "one\nline2\npartial"
	assert.True(t, len(tw.Tail()) <= limit)
	assert.True(t, bytes.HasSuffix([]byte(tw.Tail()), []byte("partial")))
}

func TestTailWriter_NoNewlineStillIncluded(t *testing.T) {
	t.Parallel()
	tw := NewTailWriter(&bytes.Buffer{}, 50)

	_, _ = tw.Write([]byte("no newline yet"))
	// Now tail contains the partial line
	assert.Equal(t, "no newline yet", tw.Tail())

	_, _ = tw.Write([]byte("\n"))
	assert.Equal(t, "no newline yet\n", tw.Tail())
}

func TestTailWriter_TrimToLimit(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tw := NewTailWriter(&buf, 5)

	_, _ = tw.Write([]byte("abcdef"))
	// Only last 5 bytes should remain
	assert.Equal(t, "bcdef", tw.Tail())
}
