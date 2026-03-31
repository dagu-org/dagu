// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package kubernetes

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromMapValidation(t *testing.T) {
	t.Run("InvalidResourceQuantityReturnsError", func(t *testing.T) {
		var (
			cfg *Config
			err error
		)

		require.NotPanics(t, func() {
			cfg, err = LoadConfigFromMap(map[string]any{
				"image": "busybox",
				"resources": map[string]any{
					"requests": map[string]any{
						"cpu": "not-a-number",
					},
				},
			})
		})

		require.Nil(t, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resources.requests.cpu")
	})

	t.Run("InvalidEmptyDirSizeLimitReturnsError", func(t *testing.T) {
		cfg, err := LoadConfigFromMap(map[string]any{
			"image": "busybox",
			"volumes": []map[string]any{
				{
					"name": "scratch",
					"empty_dir": map[string]any{
						"size_limit": "bad-size",
					},
				},
			},
		})

		require.Nil(t, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty_dir.size_limit")
	})

	t.Run("InvalidCleanupPolicyReturnsError", func(t *testing.T) {
		cfg, err := LoadConfigFromMap(map[string]any{
			"image":          "busybox",
			"cleanup_policy": "archive",
		})

		require.Nil(t, cfg)
		require.ErrorIs(t, err, ErrInvalidCleanupPolicy)
	})

	t.Run("InvalidImagePullPolicyReturnsError", func(t *testing.T) {
		cfg, err := LoadConfigFromMap(map[string]any{
			"image":             "busybox",
			"image_pull_policy": "sometimes",
		})

		require.Nil(t, cfg)
		require.ErrorIs(t, err, ErrInvalidImagePullPolicy)
	})

	t.Run("NegativeNumericFieldsReturnError", func(t *testing.T) {
		tests := []struct {
			name string
			key  string
			want error
		}{
			{name: "ActiveDeadline", key: "active_deadline", want: ErrNegativeActiveDeadline},
			{name: "BackoffLimit", key: "backoff_limit", want: ErrNegativeBackoffLimit},
			{name: "TTLAfterFinished", key: "ttl_after_finished", want: ErrNegativeTTLAfterFinished},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg, err := LoadConfigFromMap(map[string]any{
					"image": "busybox",
					tt.key:  -1,
				})

				require.Nil(t, cfg)
				require.ErrorIs(t, err, tt.want)
			})
		}
	})

	t.Run("VolumeMustDefineExactlyOneSource", func(t *testing.T) {
		cfg, err := LoadConfigFromMap(map[string]any{
			"image": "busybox",
			"volumes": []map[string]any{
				{
					"name": "mixed",
					"empty_dir": map[string]any{
						"medium": "Memory",
					},
					"secret": map[string]any{
						"secret_name": "app-secret",
					},
				},
			},
		})

		require.Nil(t, cfg)
		require.ErrorIs(t, err, ErrInvalidVolumeSource)
	})
}

func TestValidateStep(t *testing.T) {
	t.Run("RejectsInvalidConfig", func(t *testing.T) {
		err := validateStep(core.Step{
			ExecutorConfig: core.ExecutorConfig{
				Type: "kubernetes",
				Config: map[string]any{
					"image": "busybox",
					"resources": map[string]any{
						"limits": map[string]any{
							"memory": "nope",
						},
					},
				},
			},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "resources.limits.memory")
	})
}

func TestBuildCommand(t *testing.T) {
	step := core.Step{
		Shell: "/bin/sh",
		Commands: []core.CommandEntry{{
			Command:     "echo",
			Args:        []string{"hello"},
			CmdWithArgs: "echo hello",
		}},
	}

	assert.Equal(t, []string{"echo", "hello"}, buildCommand(step))
}
