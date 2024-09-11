// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSHExecutor(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		step := dag.Step{
			Name: "ssh-exec",
			ExecutorConfig: dag.ExecutorConfig{
				Type: "ssh",
				Config: map[string]any{
					"User":     "testuser",
					"IP":       "testip",
					"Port":     25,
					"Password": "testpassword",
				},
			},
		}
		ctx := context.Background()
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		sshExec, ok := exec.(*sshExec)
		require.True(t, ok)

		assert.Equal(t, "testuser", sshExec.config.User)
		assert.Equal(t, "testip", sshExec.config.IP)
		assert.Equal(t, "25", sshExec.config.Port)
		assert.Equal(t, "testpassword", sshExec.config.Password)
	})

	t.Run("ExpandEnv", func(t *testing.T) {
		os.Setenv("TEST_SSH_EXEC_USER", "testuser")
		os.Setenv("TEST_SSH_EXEC_IP", "testip")
		os.Setenv("TEST_SSH_EXEC_PORT", "23")
		os.Setenv("TEST_SSH_EXEC_PASSWORD", "testpassword")

		step := dag.Step{
			Name: "ssh-exec",
			ExecutorConfig: dag.ExecutorConfig{
				Type: "ssh",
				Config: map[string]any{
					"User":     "${TEST_SSH_EXEC_USER}",
					"IP":       "${TEST_SSH_EXEC_IP}",
					"Port":     "${TEST_SSH_EXEC_PORT}",
					"Password": "${TEST_SSH_EXEC_PASSWORD}",
				},
			},
		}
		ctx := context.Background()
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		sshExec, ok := exec.(*sshExec)
		require.True(t, ok)

		assert.Equal(t, "testuser", sshExec.config.User)
		assert.Equal(t, "testip", sshExec.config.IP)
		assert.Equal(t, "23", sshExec.config.Port)
		assert.Equal(t, "testpassword", sshExec.config.Password)
	})
}
