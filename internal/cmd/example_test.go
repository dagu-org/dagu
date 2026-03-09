// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runExampleCmd(args ...string) (string, error) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd.Example())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestExampleCommand(t *testing.T) {
	t.Run("ListAll", func(t *testing.T) {
		out, err := runExampleCmd("example")
		require.NoError(t, err)
		assert.Contains(t, out, "parallel-steps")
		assert.Contains(t, out, "agent-step")
	})

	t.Run("ShowByID", func(t *testing.T) {
		out, err := runExampleCmd("example", "1")
		require.NoError(t, err)
		assert.Contains(t, out, "type: graph")
	})

	t.Run("InvalidID", func(t *testing.T) {
		_, err := runExampleCmd("example", "99")
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("between 1 and %d", cmd.ExampleCount()))
	})

	t.Run("AllExamplesValid", func(t *testing.T) {
		for i := 1; i <= cmd.ExampleCount(); i++ {
			out, err := runExampleCmd("example", fmt.Sprintf("%d", i))
			require.NoError(t, err, "example %d failed", i)
			assert.Contains(t, out, "type: graph", "example %d missing type: graph", i)
		}
	})
}
