package s3

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextInjection(t *testing.T) {
	t.Parallel()

	t.Run("WithS3Config_and_get", func(t *testing.T) {
		t.Parallel()

		cfg := &core.S3Config{
			Region:          "us-west-2",
			Bucket:          "test-bucket",
			Endpoint:        "http://localhost:9000",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			ForcePathStyle:  true,
		}

		ctx := context.Background()
		ctx = WithS3Config(ctx, cfg)

		retrieved := getS3ConfigFromContext(ctx)
		require.NotNil(t, retrieved)
		assert.Equal(t, "us-west-2", retrieved.Region)
		assert.Equal(t, "test-bucket", retrieved.Bucket)
		assert.Equal(t, "http://localhost:9000", retrieved.Endpoint)
		assert.Equal(t, "test-key", retrieved.AccessKeyID)
		assert.Equal(t, "test-secret", retrieved.SecretAccessKey)
		assert.True(t, retrieved.ForcePathStyle)
	})

	t.Run("getS3ConfigFromContext_nil_when_not_set", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		retrieved := getS3ConfigFromContext(ctx)
		assert.Nil(t, retrieved)
	})

	t.Run("WithS3Config_nil_value", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		ctx = WithS3Config(ctx, nil)

		retrieved := getS3ConfigFromContext(ctx)
		assert.Nil(t, retrieved)
	})
}

func TestNewExecutor_DAGLevelConfigMerging(t *testing.T) {
	t.Parallel()

	t.Run("DAGLevelConfig_applied_as_defaults", func(t *testing.T) {
		t.Parallel()

		// Create DAG-level S3 config
		dagS3 := &core.S3Config{
			Region:          "us-east-1",
			Bucket:          "dag-bucket",
			Endpoint:        "http://minio:9000",
			AccessKeyID:     "dag-key",
			SecretAccessKey: "dag-secret",
			ForcePathStyle:  true,
		}

		ctx := context.Background()
		ctx = WithS3Config(ctx, dagS3)

		// Step with minimal config (just source and key for upload)
		step := core.Step{
			Name:     "upload-step",
			Commands: []core.CommandEntry{{Command: "upload"}},
			ExecutorConfig: core.ExecutorConfig{
				Type: "s3",
				Config: map[string]any{
					"source": "/tmp/test.txt",
					"key":    "uploads/test.txt",
				},
			},
		}

		exec, err := newExecutor(ctx, step)
		require.NoError(t, err)

		impl, ok := exec.(*executorImpl)
		require.True(t, ok)

		// Verify DAG-level config was applied
		assert.Equal(t, "us-east-1", impl.cfg.Region)
		assert.Equal(t, "dag-bucket", impl.cfg.Bucket)
		assert.Equal(t, "http://minio:9000", impl.cfg.Endpoint)
		assert.Equal(t, "dag-key", impl.cfg.AccessKeyID)
		assert.Equal(t, "dag-secret", impl.cfg.SecretAccessKey)
		assert.True(t, impl.cfg.ForcePathStyle)

		// Verify step-level config was also applied
		assert.Equal(t, "/tmp/test.txt", impl.cfg.Source)
		assert.Equal(t, "uploads/test.txt", impl.cfg.Key)
	})

	t.Run("StepLevelConfig_overrides_DAGLevel", func(t *testing.T) {
		t.Parallel()

		// Create DAG-level S3 config
		dagS3 := &core.S3Config{
			Region:          "us-east-1",
			Bucket:          "dag-bucket",
			Endpoint:        "http://minio:9000",
			AccessKeyID:     "dag-key",
			SecretAccessKey: "dag-secret",
		}

		ctx := context.Background()
		ctx = WithS3Config(ctx, dagS3)

		// Step with config that overrides DAG-level bucket and region
		step := core.Step{
			Name:     "upload-step",
			Commands: []core.CommandEntry{{Command: "upload"}},
			ExecutorConfig: core.ExecutorConfig{
				Type: "s3",
				Config: map[string]any{
					"source": "/tmp/test.txt",
					"key":    "uploads/test.txt",
					"bucket": "step-bucket", // Override
					"region": "eu-west-1",   // Override
				},
			},
		}

		exec, err := newExecutor(ctx, step)
		require.NoError(t, err)

		impl, ok := exec.(*executorImpl)
		require.True(t, ok)

		// Step-level overrides
		assert.Equal(t, "step-bucket", impl.cfg.Bucket)
		assert.Equal(t, "eu-west-1", impl.cfg.Region)

		// DAG-level values for non-overridden fields
		assert.Equal(t, "http://minio:9000", impl.cfg.Endpoint)
		assert.Equal(t, "dag-key", impl.cfg.AccessKeyID)
		assert.Equal(t, "dag-secret", impl.cfg.SecretAccessKey)
	})

	t.Run("NoDAGLevelConfig_uses_step_only", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// No DAG-level config set

		step := core.Step{
			Name:     "upload-step",
			Commands: []core.CommandEntry{{Command: "upload"}},
			ExecutorConfig: core.ExecutorConfig{
				Type: "s3",
				Config: map[string]any{
					"source":          "/tmp/test.txt",
					"key":             "uploads/test.txt",
					"bucket":          "step-bucket",
					"region":          "ap-northeast-1",
					"accessKeyId":     "step-key",
					"secretAccessKey": "step-secret",
				},
			},
		}

		exec, err := newExecutor(ctx, step)
		require.NoError(t, err)

		impl, ok := exec.(*executorImpl)
		require.True(t, ok)

		assert.Equal(t, "step-bucket", impl.cfg.Bucket)
		assert.Equal(t, "ap-northeast-1", impl.cfg.Region)
		assert.Equal(t, "step-key", impl.cfg.AccessKeyID)
		assert.Equal(t, "step-secret", impl.cfg.SecretAccessKey)
	})

	t.Run("DAGLevelConfig_partial_override", func(t *testing.T) {
		t.Parallel()

		// Create DAG-level S3 config with all fields
		dagS3 := &core.S3Config{
			Region:          "us-west-2",
			Bucket:          "dag-bucket",
			Endpoint:        "http://localhost:9000",
			AccessKeyID:     "dag-key",
			SecretAccessKey: "dag-secret",
			SessionToken:    "dag-token",
			ForcePathStyle:  true,
			DisableSSL:      true,
		}

		ctx := context.Background()
		ctx = WithS3Config(ctx, dagS3)

		// Step only overrides endpoint and forcePathStyle
		step := core.Step{
			Name:     "list-step",
			Commands: []core.CommandEntry{{Command: "list"}},
			ExecutorConfig: core.ExecutorConfig{
				Type: "s3",
				Config: map[string]any{
					"endpoint":       "http://production-s3:9000",
					"forcePathStyle": false,
				},
			},
		}

		exec, err := newExecutor(ctx, step)
		require.NoError(t, err)

		impl, ok := exec.(*executorImpl)
		require.True(t, ok)

		// Step-level overrides
		assert.Equal(t, "http://production-s3:9000", impl.cfg.Endpoint)
		assert.False(t, impl.cfg.ForcePathStyle)

		// DAG-level values preserved
		assert.Equal(t, "us-west-2", impl.cfg.Region)
		assert.Equal(t, "dag-bucket", impl.cfg.Bucket)
		assert.Equal(t, "dag-key", impl.cfg.AccessKeyID)
		assert.Equal(t, "dag-secret", impl.cfg.SecretAccessKey)
		assert.Equal(t, "dag-token", impl.cfg.SessionToken)
		assert.True(t, impl.cfg.DisableSSL)
	})
}

func TestNewExecutor_ValidationWithDAGConfig(t *testing.T) {
	t.Parallel()

	t.Run("DAGLevelBucket_satisfies_validation", func(t *testing.T) {
		t.Parallel()

		// DAG-level config provides bucket
		dagS3 := &core.S3Config{
			Bucket: "dag-bucket",
		}

		ctx := context.Background()
		ctx = WithS3Config(ctx, dagS3)

		// Step doesn't specify bucket (uses DAG-level)
		step := core.Step{
			Name:     "list-step",
			Commands: []core.CommandEntry{{Command: "list"}},
			ExecutorConfig: core.ExecutorConfig{
				Type:   "s3",
				Config: map[string]any{},
			},
		}

		exec, err := newExecutor(ctx, step)
		require.NoError(t, err)

		impl, ok := exec.(*executorImpl)
		require.True(t, ok)
		assert.Equal(t, "dag-bucket", impl.cfg.Bucket)
	})

	t.Run("MissingBucket_fails_validation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// No DAG-level config

		step := core.Step{
			Name:     "list-step",
			Commands: []core.CommandEntry{{Command: "list"}},
			ExecutorConfig: core.ExecutorConfig{
				Type:   "s3",
				Config: map[string]any{},
			},
		}

		_, err := newExecutor(ctx, step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bucket")
	})
}

func TestValidateStep(t *testing.T) {
	t.Parallel()

	t.Run("valid_command", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "s3"},
			Commands:       []core.CommandEntry{{Command: "upload"}},
		}
		err := validateStep(step)
		require.NoError(t, err)
	})

	t.Run("empty_command", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "s3"},
			Commands:       []core.CommandEntry{{Command: ""}},
		}
		err := validateStep(step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("no_commands", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "s3"},
			Commands:       []core.CommandEntry{},
		}
		err := validateStep(step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("different_executor_type_skipped", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			ExecutorConfig: core.ExecutorConfig{Type: "http"},
			Commands:       []core.CommandEntry{},
		}
		err := validateStep(step)
		require.NoError(t, err)
	})
}

func TestNewExecutor_Operations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		command   string
		config    map[string]any
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "upload_valid",
			command: "upload",
			config: map[string]any{
				"bucket": "test-bucket",
				"source": "/tmp/file.txt",
				"key":    "uploads/file.txt",
			},
		},
		{
			name:    "download_valid",
			command: "download",
			config: map[string]any{
				"bucket":      "test-bucket",
				"key":         "downloads/file.txt",
				"destination": "/tmp/file.txt",
			},
		},
		{
			name:    "list_valid",
			command: "list",
			config: map[string]any{
				"bucket": "test-bucket",
			},
		},
		{
			name:    "delete_valid_with_key",
			command: "delete",
			config: map[string]any{
				"bucket": "test-bucket",
				"key":    "delete/file.txt",
			},
		},
		{
			name:    "delete_valid_with_prefix",
			command: "delete",
			config: map[string]any{
				"bucket": "test-bucket",
				"prefix": "delete/",
			},
		},
		{
			name:      "invalid_operation",
			command:   "copy",
			config:    map[string]any{"bucket": "test-bucket"},
			wantErr:   true,
			errSubstr: "unsupported s3 operation",
		},
		{
			name:      "upload_missing_source",
			command:   "upload",
			config:    map[string]any{"bucket": "test-bucket", "key": "test.txt"},
			wantErr:   true,
			errSubstr: "source is required",
		},
		{
			name:      "upload_missing_key",
			command:   "upload",
			config:    map[string]any{"bucket": "test-bucket", "source": "/tmp/file.txt"},
			wantErr:   true,
			errSubstr: "key is required",
		},
		{
			name:      "download_missing_destination",
			command:   "download",
			config:    map[string]any{"bucket": "test-bucket", "key": "file.txt"},
			wantErr:   true,
			errSubstr: "destination is required",
		},
		{
			name:      "delete_missing_key_and_prefix",
			command:   "delete",
			config:    map[string]any{"bucket": "test-bucket"},
			wantErr:   true,
			errSubstr: "key or prefix is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := core.Step{
				Name:     tt.name,
				Commands: []core.CommandEntry{{Command: tt.command}},
				ExecutorConfig: core.ExecutorConfig{
					Type:   "s3",
					Config: tt.config,
				},
			}

			_, err := newExecutor(context.Background(), step)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	assert.Equal(t, int64(10), cfg.PartSize)
	assert.Equal(t, 5, cfg.Concurrency)
	assert.Equal(t, 1000, cfg.MaxKeys)
	assert.Equal(t, "json", cfg.OutputFormat)
}
