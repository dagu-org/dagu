package spec_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStepTimeout(t *testing.T) {
	t.Parallel()

	// Positive timeout
	t.Run("PositiveTimeout", func(t *testing.T) {
		data := []byte(`
steps:
  - name: work
    command: echo doing
    timeoutSec: 5
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, 5*time.Second, dag.Steps[0].Timeout)
	})

	// Zero timeout (explicit) -> unset/zero duration
	t.Run("ZeroTimeoutExplicit", func(t *testing.T) {
		data := []byte(`
steps:
  - name: work
    command: echo none
    timeoutSec: 0
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, time.Duration(0), dag.Steps[0].Timeout)
	})

	// Zero timeout (omitted) -> also zero
	t.Run("ZeroTimeoutOmitted", func(t *testing.T) {
		data := []byte(`
steps:
  - name: work
    command: echo omitted
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, time.Duration(0), dag.Steps[0].Timeout)
	})

	// Negative timeout should fail validation
	t.Run("NegativeTimeout", func(t *testing.T) {
		data := []byte(`
steps:
  - name: bad
    command: echo fail
    timeoutSec: -3
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeoutSec must be >= 0")
	})
}
