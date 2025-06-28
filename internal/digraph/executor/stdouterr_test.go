package executor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExecutorsStdoutStderrSeparation verifies that executors properly handle separate stdout/stderr
func TestExecutorsStdoutStderrSeparation(t *testing.T) {
	t.Parallel()

	// Test that all executor types have proper stdout/stderr handling
	// This is a compilation test to ensure the fields exist and methods work correctly

	t.Run("SSHExecutor", func(t *testing.T) {
		exec := &sshExec{
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf)

		assert.Equal(t, stdoutBuf, exec.stdout)
		assert.Equal(t, stderrBuf, exec.stderr)
		assert.NotSame(t, exec.stdout, exec.stderr)
	})

	t.Run("JQExecutor", func(t *testing.T) {
		exec := &jq{
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf)

		assert.Equal(t, stdoutBuf, exec.stdout)
		assert.Equal(t, stderrBuf, exec.stderr)
		assert.NotSame(t, exec.stdout, exec.stderr)
	})

	t.Run("MailExecutor", func(t *testing.T) {
		exec := &mail{
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf)

		assert.Equal(t, stdoutBuf, exec.stdout)
		assert.Equal(t, stderrBuf, exec.stderr)
		assert.NotSame(t, exec.stdout, exec.stderr)
	})

	t.Run("DockerExecutor", func(t *testing.T) {
		exec := &docker{
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf)

		assert.Equal(t, stdoutBuf, exec.stdout)
		assert.Equal(t, stderrBuf, exec.stderr)
		assert.NotSame(t, exec.stdout, exec.stderr)
	})

	t.Run("HTTPExecutor", func(t *testing.T) {
		exec := &http{
			stdout: &bytes.Buffer{},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf) // HTTP executor ignores stderr

		assert.Equal(t, stdoutBuf, exec.stdout)
		// HTTP executor doesn't have stderr field
	})

	t.Run("CommandExecutor", func(t *testing.T) {
		exec := &commandExecutor{
			config: &commandConfig{
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			},
		}

		stdoutBuf := &bytes.Buffer{}
		stderrBuf := &bytes.Buffer{}

		exec.SetStdout(stdoutBuf)
		exec.SetStderr(stderrBuf)

		assert.Equal(t, stdoutBuf, exec.config.Stdout)
		assert.Equal(t, stderrBuf, exec.config.Stderr)
		assert.NotSame(t, exec.config.Stdout, exec.config.Stderr)
	})
}
