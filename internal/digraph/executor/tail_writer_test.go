package executor

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestTailWriter_LastLineSimple(t *testing.T) {
    t.Parallel()
    var buf bytes.Buffer
    tw := newTailWriter(&buf)

    input := "line1\nline2\n"
    n, err := tw.Write([]byte(input))
    assert.NoError(t, err)
    assert.Equal(t, len(input), n)

    // Underlying receives full content
    assert.Equal(t, input, buf.String())
    // Tail returns last completed line
    assert.Equal(t, "line2", tw.Tail())
}

func TestTailWriter_AcrossWrites(t *testing.T) {
    t.Parallel()
    var buf bytes.Buffer
    tw := newTailWriter(&buf)

    _, _ = tw.Write([]byte("one\nlin"))
    assert.Equal(t, "one", tw.Tail()) // first completed line

    _, _ = tw.Write([]byte("e2\npar"))
    assert.Equal(t, "line2", tw.Tail()) // completes second line

    _, _ = tw.Write([]byte("tial"))
    assert.Equal(t, "line2", tw.Tail()) // still last completed line

    _, _ = tw.Write([]byte("\n"))
    assert.Equal(t, "partial", tw.Tail()) // new completed line
}

func TestTailWriter_NoCompletedLineYet(t *testing.T) {
    t.Parallel()
    tw := newTailWriter(&bytes.Buffer{})

    _, _ = tw.Write([]byte("no newline yet"))
    assert.Equal(t, "", tw.Tail()) // no completed line

    _, _ = tw.Write([]byte("\n"))
    assert.Equal(t, "no newline yet", tw.Tail())
}

func TestTailWriter_EmptyLineHandling(t *testing.T) {
    t.Parallel()
    tw := newTailWriter(&bytes.Buffer{})

    _, _ = tw.Write([]byte("err\n\nlast\n"))
    assert.Equal(t, "last", tw.Tail())
}

