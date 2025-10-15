package masking

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskingWriter_SingleWrite(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	_, err := writer.Write([]byte("The key is secret123\n"))
	require.NoError(t, err)

	assert.Equal(t, "The key is *******\n", buf.String())
}

func TestMaskingWriter_MultipleLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	_, err := writer.Write([]byte("Line 1: secret123\nLine 2: secret123\n"))
	require.NoError(t, err)

	assert.Equal(t, "Line 1: *******\nLine 2: *******\n", buf.String())
}

func TestMaskingWriter_SplitAcrossWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write "The key is " (no newline)
	_, err := writer.Write([]byte("The key is "))
	require.NoError(t, err)
	assert.Empty(t, buf.String()) // Nothing written yet (no newline)

	// Write "secret123\n" (completes the line)
	_, err = writer.Write([]byte("secret123\n"))
	require.NoError(t, err)
	assert.Equal(t, "The key is *******\n", buf.String())
}

func TestMaskingWriter_SecretSplitAcrossWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write "The key is sec" (splits the secret)
	_, err := writer.Write([]byte("The key is sec"))
	require.NoError(t, err)
	assert.Empty(t, buf.String())

	// Write "ret123\n" (completes the secret and line)
	_, err = writer.Write([]byte("ret123\n"))
	require.NoError(t, err)
	assert.Equal(t, "The key is *******\n", buf.String())
}

func TestMaskingWriter_NoNewline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write without newline
	_, err := writer.Write([]byte("The key is secret123"))
	require.NoError(t, err)
	assert.Empty(t, buf.String()) // Buffered, not written yet

	// Flush to write buffered data
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "The key is *******", buf.String())
}

func TestMaskingWriter_MultipleSecrets(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{
			"API_KEY=secret123",
			"PASSWORD=pass456",
		},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	_, err := writer.Write([]byte("API: secret123, PWD: pass456\n"))
	require.NoError(t, err)

	assert.Equal(t, "API: *******, PWD: *******\n", buf.String())
}

func TestMaskingWriter_NilMasker(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writer := NewMaskingWriter(&buf, nil)

	input := "The key is secret123\n"
	_, err := writer.Write([]byte(input))
	require.NoError(t, err)

	// Should pass through without masking
	assert.Equal(t, input, buf.String())
}

func TestMaskingWriter_EmptyWrite(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	n, err := writer.Write([]byte{})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestMaskingWriter_Flush(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write without newline
	_, err := writer.Write([]byte("secret123"))
	require.NoError(t, err)
	assert.Empty(t, buf.String())

	// Flush should write buffered data
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "*******", buf.String())

	// Second flush should be no-op
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "*******", buf.String())
}

func TestMaskingWriter_Close(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write without newline
	_, err := writer.Write([]byte("secret123"))
	require.NoError(t, err)

	// Close should flush
	err = writer.Close()
	require.NoError(t, err)
	assert.Equal(t, "*******", buf.String())
}

func TestMaskingWriter_CloseWithCloser(t *testing.T) {
	t.Parallel()

	// Create a writer that implements io.Closer
	closerWriter := &mockCloser{buf: &bytes.Buffer{}}
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(closerWriter, masker)

	_, err := writer.Write([]byte("secret123"))
	require.NoError(t, err)

	// Close should flush and call underlying Close
	err = writer.Close()
	require.NoError(t, err)
	assert.True(t, closerWriter.closed)
	assert.Equal(t, "*******", closerWriter.buf.String())
}

func TestMaskingWriter_LargeWrite(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Generate large input with secrets scattered throughout
	var input strings.Builder
	for i := 0; i < 1000; i++ {
		input.WriteString("Line ")
		input.WriteString(string(rune('0' + (i % 10))))
		input.WriteString(": The secret is secret123\n")
	}

	_, err := writer.Write([]byte(input.String()))
	require.NoError(t, err)

	// Verify all secrets are masked
	output := buf.String()
	assert.NotContains(t, output, "secret123")
	assert.Contains(t, output, "*******")
}

func TestMaskingWriter_PartialLineBuffer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}
	masker := NewMasker(sources)
	writer := NewMaskingWriter(&buf, masker)

	// Write line with newline
	_, err := writer.Write([]byte("Complete line\n"))
	require.NoError(t, err)

	// Write partial line without newline
	_, err = writer.Write([]byte("Partial secret123"))
	require.NoError(t, err)

	// Only complete line should be written
	assert.Equal(t, "Complete line\n", buf.String())

	// Flush to get partial line
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "Complete line\nPartial *******", buf.String())
}

// mockCloser is a mock writer that implements io.WriteCloser
type mockCloser struct {
	buf    *bytes.Buffer
	closed bool
}

func (m *mockCloser) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockCloser) Close() error {
	m.closed = true
	return nil
}

var _ io.WriteCloser = (*mockCloser)(nil)
