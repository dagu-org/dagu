package cmd_test

import (
	"bytes"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/dagu-org/dagu/internal/agent/schema" // Register schemas
)

func runSchemaCmd(args ...string) (string, error) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd.Schema())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestSchemaCommand(t *testing.T) {
	t.Run("DAGRoot", func(t *testing.T) {
		_, err := runSchemaCmd("schema", "dag")
		require.NoError(t, err)
	})

	t.Run("ConfigRoot", func(t *testing.T) {
		_, err := runSchemaCmd("schema", "config")
		require.NoError(t, err)
	})

	t.Run("DAGSteps", func(t *testing.T) {
		_, err := runSchemaCmd("schema", "dag", "steps")
		require.NoError(t, err)
	})

	t.Run("InvalidSchema", func(t *testing.T) {
		_, err := runSchemaCmd("schema", "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "available")
	})

	t.Run("InvalidPath", func(t *testing.T) {
		_, err := runSchemaCmd("schema", "dag", "nonexistent.path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("NoArgs", func(t *testing.T) {
		_, err := runSchemaCmd("schema")
		require.Error(t, err)
	})
}
