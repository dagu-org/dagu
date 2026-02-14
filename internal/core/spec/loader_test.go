package spec_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("WithName", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG, spec.WithName("testDAG"))
		require.NoError(t, err)
		require.Equal(t, "testDAG", dag.Name)
	})

	// Error cases
	errorTests := []struct {
		name        string
		content     string
		useFile     bool
		errContains string
	}{
		{
			name:    "InvalidPath",
			useFile: false,
		},
		{
			name:        "UnknownField",
			content:     "invalidKey: test\n",
			useFile:     true,
			errContains: "has invalid keys: invalidKey",
		},
		{
			name:        "InvalidYAML",
			content:     "invalidyaml",
			useFile:     true,
			errContains: "invalidyaml",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var testDAG string
			if tt.useFile {
				testDAG = createTempYAMLFile(t, tt.content)
			} else {
				testDAG = "/tmp/non_existing_file_" + t.Name() + ".yaml"
			}
			_, err := spec.Load(context.Background(), testDAG)
			require.Error(t, err)
			if tt.errContains != "" {
				require.Contains(t, err.Error(), tt.errContains)
			}
		})
	}

	t.Run("MetadataOnly", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
log_dir: /var/log/dagu
hist_retention_days: 90
max_clean_up_time_sec: 60
mail_on:
  failure: true
steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG, spec.OnlyMetadata())
		require.NoError(t, err)
		// Steps should not be loaded in metadata-only mode
		require.Empty(t, dag.Steps)
		// Non-metadata fields from YAML should NOT be populated in metadata-only mode
		assert.Empty(t, dag.LogDir, "LogDir should be empty in metadata-only mode")
		assert.Nil(t, dag.MailOn, "MailOn should be nil in metadata-only mode")
		// Defaults are still applied by InitializeDefaults (not from YAML)
		assert.Equal(t, 30, dag.HistRetentionDays, "HistRetentionDays should be default (30), not YAML value (90)")
		assert.Equal(t, 5*time.Second, dag.MaxCleanUpTime, "MaxCleanUpTime should be default (5s), not YAML value (60s)")
	})
	t.Run("DefaultConfig", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG)

		require.NoError(t, err)

		// DAG level
		assert.Equal(t, "", dag.LogDir)
		assert.Equal(t, testDAG, dag.Location)
		assert.Equal(t, time.Second*5, dag.MaxCleanUpTime)
		assert.Equal(t, 30, dag.HistRetentionDays)

		// Step level
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "1", dag.Steps[0].Name, "1")
		require.Len(t, dag.Steps[0].Commands, 1)
		assert.Equal(t, "true", dag.Steps[0].Commands[0].Command)
	})
	t.Run("OverrideConfig", func(t *testing.T) {
		t.Parallel()

		// Base config has the following values:
		// MailOn: {Failure: true, Success: false}
		base := createTempYAMLFile(t, `env:
  LOG_DIR: "${HOME}/logs"
log_dir: "${LOG_DIR}"
smtp:
  host: "smtp.host"
  port: "25"
error_mail:
  from: "system@mail.com"
  to: "error@mail.com"
  prefix: "[ERROR]"
info_mail:
  from: "system@mail.com"
  to: "info@mail.com"
  prefix: "[INFO]"
mail_on:
  failure: true
`)
		// Overwrite the base config with the following values:
		// MailOn: {Failure: false, Success: true}
		testDAG := createTempYAMLFile(t, `mail_on:
  failure: false
  success: true

hist_retention_days: 7

steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG, spec.WithBaseConfig(base))
		require.NoError(t, err)

		// Check if the MailOn values are overwritten
		assert.False(t, dag.MailOn.Failure)
		assert.True(t, dag.MailOn.Success)
	})
	t.Run("OverrideErrorMailPrefixOnly", func(t *testing.T) {
		t.Parallel()

		// Base config has error_mail with all fields set
		base := createTempYAMLFile(t, `error_mail:
  from: "base@example.com"
  to: "error@example.com"
  prefix: "[BASE-ERROR]"
  attach_logs: true
info_mail:
  from: "base@example.com"
  to: "info@example.com"
  prefix: "[BASE-INFO]"
wait_mail:
  from: "base@example.com"
  to: "wait@example.com"
  prefix: "[BASE-WAIT]"
`)
		// Child DAG only overrides prefix - this should work without
		// requiring other fields to be specified (GitHub issue #1512)
		testDAG := createTempYAMLFile(t, `error_mail:
  prefix: "[OVERRIDE-ERROR]"
info_mail:
  prefix: "[OVERRIDE-INFO]"
wait_mail:
  prefix: "[OVERRIDE-WAIT]"

steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG, spec.WithBaseConfig(base))
		require.NoError(t, err)

		// Check if error_mail prefix is overridden
		require.NotNil(t, dag.ErrorMail)
		assert.Equal(t, "[OVERRIDE-ERROR]", dag.ErrorMail.Prefix, "error_mail prefix should be overridden")
		// Other fields should be inherited from base
		assert.Equal(t, "base@example.com", dag.ErrorMail.From, "error_mail from should be inherited from base")
		assert.Equal(t, []string{"error@example.com"}, dag.ErrorMail.To, "error_mail to should be inherited from base")
		assert.True(t, dag.ErrorMail.AttachLogs, "error_mail attach_logs should be inherited from base")

		// Check if info_mail prefix is overridden
		require.NotNil(t, dag.InfoMail)
		assert.Equal(t, "[OVERRIDE-INFO]", dag.InfoMail.Prefix, "info_mail prefix should be overridden")
		// Other fields should be inherited from base
		assert.Equal(t, "base@example.com", dag.InfoMail.From, "info_mail from should be inherited from base")
		assert.Equal(t, []string{"info@example.com"}, dag.InfoMail.To, "info_mail to should be inherited from base")

		// Check if wait_mail prefix is overridden
		require.NotNil(t, dag.WaitMail)
		assert.Equal(t, "[OVERRIDE-WAIT]", dag.WaitMail.Prefix, "wait_mail prefix should be overridden")
		// Other fields should be inherited from base
		assert.Equal(t, "base@example.com", dag.WaitMail.From, "wait_mail from should be inherited from base")
		assert.Equal(t, []string{"wait@example.com"}, dag.WaitMail.To, "wait_mail to should be inherited from base")
	})
	t.Run("OverrideSMTPCredentialsOnly", func(t *testing.T) {
		t.Parallel()

		// Base config has smtp with host and port set
		base := createTempYAMLFile(t, `smtp:
  host: "smtp.base.com"
  port: "587"
`)
		// Child DAG only overrides username and password
		// This should work and inherit host/port from base
		testDAG := createTempYAMLFile(t, `smtp:
  username: "override-user"
  password: "override-pass"

steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), testDAG, spec.WithBaseConfig(base))
		require.NoError(t, err)

		// Check if SMTP username and password are set (from child)
		require.NotNil(t, dag.SMTP)
		assert.Equal(t, "override-user", dag.SMTP.Username, "smtp username should be set")
		assert.Equal(t, "override-pass", dag.SMTP.Password, "smtp password should be set")
		// Host and port should be inherited from base
		assert.Equal(t, "smtp.base.com", dag.SMTP.Host, "smtp host should be inherited from base")
		assert.Equal(t, "587", dag.SMTP.Port, "smtp port should be inherited from base")
	})
	t.Run("WaitMailConfig", func(t *testing.T) {
		t.Parallel()

		// Test wait_mail loading independently with all fields
		dagFile := createTempYAMLFile(t, `
wait_mail:
  from: "wait@example.com"
  to:
    - "approvers@example.com"
    - "managers@example.com"
  prefix: "[APPROVAL REQUIRED]"
  attach_logs: false

mail_on:
  wait: true

steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), dagFile)
		require.NoError(t, err)

		require.NotNil(t, dag.WaitMail)
		assert.Equal(t, "wait@example.com", dag.WaitMail.From)
		assert.Equal(t, []string{"approvers@example.com", "managers@example.com"}, dag.WaitMail.To)
		assert.Equal(t, "[APPROVAL REQUIRED]", dag.WaitMail.Prefix)
		assert.False(t, dag.WaitMail.AttachLogs)

		require.NotNil(t, dag.MailOn)
		assert.True(t, dag.MailOn.Wait)
	})
	t.Run("WaitMailSingleRecipient", func(t *testing.T) {
		t.Parallel()

		// Test wait_mail with single recipient (string format)
		dagFile := createTempYAMLFile(t, `
wait_mail:
  from: "wait@example.com"
  to: "single@example.com"
  prefix: "[WAIT]"

steps:
  - name: "1"
    command: "true"
`)
		dag, err := spec.Load(context.Background(), dagFile)
		require.NoError(t, err)

		require.NotNil(t, dag.WaitMail)
		assert.Equal(t, "wait@example.com", dag.WaitMail.From)
		assert.Equal(t, []string{"single@example.com"}, dag.WaitMail.To)
		assert.Equal(t, "[WAIT]", dag.WaitMail.Prefix)
		assert.False(t, dag.WaitMail.AttachLogs)
	})
}

func TestLoadBaseConfig(t *testing.T) {
	t.Parallel()

	t.Run("LoadBaseConfigFile", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `env:
  LOG_DIR: "${HOME}/logs"
log_dir: "${LOG_DIR}"
smtp:
  host: "smtp.host"
  port: "25"
error_mail:
  from: "system@mail.com"
  to: "error@mail.com"
  prefix: "[ERROR]"
info_mail:
  from: "system@mail.com"
  to: "info@mail.com"
  prefix: "[INFO]"
mail_on:
  failure: true
`)
		dag, err := spec.LoadBaseConfig(spec.BuildContext{}, testDAG)
		require.NotNil(t, dag)
		require.NoError(t, err)
	})
	t.Run("InheritBaseConfig", func(t *testing.T) {
		t.Parallel()

		// Base config with multiple inheritable fields
		baseDAG := createTempYAMLFile(t, `
env:
  BASE_ENV: "base_value"
  OVERWRITE_ENV: "base_overwrite_value"

log_dir: "/base/logs"
log_output: merged
hist_retention_days: 90
max_clean_up_time_sec: 120

llm:
  provider: openai
  model: gpt-4o
  system: "Base system prompt"
`)

		// Child DAG inherits all base config fields
		childDAG := createTempYAMLFile(t, `
env:
  SUB_ENV: "sub_value"
  OVERWRITE_ENV: "sub_overwrite_value"

steps:
  - name: "step1"
    command: echo "step1"
`)
		dag, err := spec.Load(context.Background(), childDAG, spec.WithBaseConfig(baseDAG))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Env inheritance: base + child merged, child overrides base
		assert.Contains(t, dag.Env, "BASE_ENV=base_value")
		assert.Contains(t, dag.Env, "SUB_ENV=sub_value")
		assert.Contains(t, dag.Env, "OVERWRITE_ENV=sub_overwrite_value")

		// LogDir inherited from base
		assert.Equal(t, "/base/logs", dag.LogDir)

		// LogOutput inherited from base
		assert.Equal(t, core.LogOutputMerged, dag.LogOutput)

		// HistRetentionDays inherited from base
		assert.Equal(t, 90, dag.HistRetentionDays)

		// MaxCleanUpTime inherited from base
		assert.Equal(t, 120*time.Second, dag.MaxCleanUpTime)

		// LLM inherited from base
		require.NotNil(t, dag.LLM)
		assert.Equal(t, "openai", dag.LLM.Provider)
		assert.Equal(t, "gpt-4o", dag.LLM.Model)
		assert.Equal(t, "Base system prompt", dag.LLM.System)
	})

	t.Run("OverrideBaseConfig", func(t *testing.T) {
		t.Parallel()

		// Base config with multiple inheritable fields
		baseDAG := createTempYAMLFile(t, `
env:
  BASE_ENV: "base_value"

log_dir: "/base/logs"
log_output: merged
hist_retention_days: 90
max_clean_up_time_sec: 120

llm:
  provider: openai
  model: gpt-4o
  system: "Base system prompt"
`)

		// Child DAG overrides specific fields
		overrideDAG := createTempYAMLFile(t, `
log_dir: "/override/logs"
log_output: separate
hist_retention_days: 7
max_clean_up_time_sec: 30

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  system: "Override system prompt"

steps:
  - name: "step1"
    command: echo "step1"
`)
		dag, err := spec.Load(context.Background(), overrideDAG, spec.WithBaseConfig(baseDAG))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// LogDir overridden
		assert.Equal(t, "/override/logs", dag.LogDir)

		// LogOutput overridden
		assert.Equal(t, core.LogOutputSeparate, dag.LogOutput)

		// HistRetentionDays overridden
		assert.Equal(t, 7, dag.HistRetentionDays)

		// MaxCleanUpTime overridden
		assert.Equal(t, 30*time.Second, dag.MaxCleanUpTime)

		// LLM overridden
		require.NotNil(t, dag.LLM)
		assert.Equal(t, "anthropic", dag.LLM.Provider)
		assert.Equal(t, "claude-sonnet-4-20250514", dag.LLM.Model)
		assert.Equal(t, "Override system prompt", dag.LLM.System)

		// Env still inherited from base (since not specified in override DAG)
		assert.Contains(t, dag.Env, "BASE_ENV=base_value")
	})

	t.Run("InheritBaseWorkingDir", func(t *testing.T) {
		t.Parallel()

		baseDAG := createTempYAMLFile(t, `
working_dir: /shared/workspace
`)

		childDAG := createTempYAMLFile(t, `
steps:
  - name: "step1"
    command: echo "test"
`)

		dag, err := spec.Load(context.Background(), childDAG, spec.WithBaseConfig(baseDAG))
		require.NoError(t, err)

		// Child should inherit base's working_dir
		assert.Equal(t, "/shared/workspace", dag.WorkingDir)
	})

	t.Run("OverrideBaseWorkingDir", func(t *testing.T) {
		t.Parallel()

		baseDAG := createTempYAMLFile(t, `
working_dir: /shared/workspace
`)

		childDAG := createTempYAMLFile(t, `
working_dir: /my/custom/dir
steps:
  - name: "step1"
    command: echo "test"
`)

		dag, err := spec.Load(context.Background(), childDAG, spec.WithBaseConfig(baseDAG))
		require.NoError(t, err)

		// Child's explicit working_dir should override base
		assert.Equal(t, "/my/custom/dir", dag.WorkingDir)
	})
}

func TestLoadYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantName    string
		wantCommand string
	}{
		{
			name: "ValidYAMLData",
			input: `steps:
  - name: "1"
    command: "true"
`,
			wantName:    "1",
			wantCommand: "true",
		},
		{
			name:    "InvalidYAMLData",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ret, err := spec.LoadYAMLWithOpts(context.Background(), []byte(tt.input), spec.BuildOpts{})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, ret.Steps, 1)
			assert.Equal(t, tt.wantName, ret.Steps[0].Name)
			require.Len(t, ret.Steps[0].Commands, 1)
			assert.Equal(t, tt.wantCommand, ret.Steps[0].Commands[0].Command)
		})
	}
}

func TestLoadYAMLWithNameOption(t *testing.T) {
	t.Parallel()
	const testDAG = `
steps:
  - name: "1"
    command: "true"
`

	ret, err := spec.LoadYAMLWithOpts(context.Background(), []byte(testDAG), spec.BuildOpts{
		Name: "testDAG",
	})
	require.NoError(t, err)
	require.Equal(t, "testDAG", ret.Name)

	step := ret.Steps[0]
	require.Equal(t, "1", step.Name)
	require.Len(t, step.Commands, 1)
	require.Equal(t, "true", step.Commands[0].Command)
}

// createTempYAMLFile creates a temporary YAML file with the given content
func createTempYAMLFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	_, err = tmpFile.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

func TestMultiDAGFile(t *testing.T) {
	t.Parallel()

	t.Run("LoadMultipleDAGs", func(t *testing.T) {
		t.Parallel()

		// Create a temporary multi-DAG YAML file
		multiDAGContent := `steps:
  - name: process
    call: transform-data
  - name: archive
    call: archive-results

---
name: transform-data
steps:
  - name: transform
    command: transform.py

---
name: archive-results
steps:
  - name: archive
    command: archive.sh
`
		// Create temporary file
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load the multi-DAG file
		dag, err := spec.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify main DAG
		assert.Len(t, dag.Steps, 2)
		assert.Equal(t, "process", dag.Steps[0].Name)
		assert.Equal(t, "transform-data", dag.Steps[0].SubDAG.Name)
		assert.Equal(t, "archive", dag.Steps[1].Name)
		assert.Equal(t, "archive-results", dag.Steps[1].SubDAG.Name)

		// Verify sub DAGs
		require.NotNil(t, dag.LocalDAGs)
		assert.Len(t, dag.LocalDAGs, 2)

		// Check transform-data sub DAG
		_, exists := dag.LocalDAGs["transform-data"]
		require.True(t, exists)
		transformDAG := dag.LocalDAGs["transform-data"]
		assert.Equal(t, "transform-data", transformDAG.Name)
		assert.Len(t, transformDAG.Steps, 1)
		assert.Equal(t, "transform", transformDAG.Steps[0].Name)
		require.Len(t, transformDAG.Steps[0].Commands, 1)
		assert.Equal(t, "transform.py", transformDAG.Steps[0].Commands[0].Command)

		// Check archive-results sub DAG
		_, exists = dag.LocalDAGs["archive-results"]
		require.True(t, exists)
		archiveDAG := dag.LocalDAGs["archive-results"]
		assert.Equal(t, "archive-results", archiveDAG.Name)
		assert.Len(t, archiveDAG.Steps, 1)
		assert.Equal(t, "archive", archiveDAG.Steps[0].Name)
		require.Len(t, archiveDAG.Steps[0].Commands, 1)
		assert.Equal(t, "archive.sh", archiveDAG.Steps[0].Commands[0].Command)
	})

	t.Run("MultiDAGWithBaseConfig", func(t *testing.T) {
		t.Parallel()

		// Create base config
		baseConfig := `env:
  - ENV: production
  - API_KEY: secret123
log_dir: /base/logs
smtp:
  host: smtp.example.com
  port: "587"
`
		baseFile := createTempYAMLFile(t, baseConfig)

		// Create multi-DAG file
		multiDAGContent := `env:
  - APP: myapp
steps:
  - name: process
    command: echo "main"

---
name: sub-dag
env:
  - SERVICE: worker
steps:
  - name: work
    command: echo "child"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load with base config
		dag, err := spec.Load(context.Background(), tmpFile,
			spec.WithBaseConfig(baseFile))
		require.NoError(t, err)

		// Verify main DAG inherits base config
		assert.Equal(t, "/base/logs", dag.LogDir)
		assert.NotNil(t, dag.SMTP)
		assert.Equal(t, "smtp.example.com", dag.SMTP.Host)

		// Verify main DAG has merged env vars
		assert.Contains(t, dag.Env, "ENV=production")
		assert.Contains(t, dag.Env, "API_KEY=secret123")
		assert.Contains(t, dag.Env, "APP=myapp")

		// Verify sub DAG also inherits base config
		subDAG := dag.LocalDAGs["sub-dag"]
		require.NotNil(t, subDAG)
		assert.Equal(t, "/base/logs", subDAG.LogDir)
		assert.NotNil(t, subDAG.SMTP)
		assert.Equal(t, "smtp.example.com", subDAG.SMTP.Host)

		// Verify sub DAG has merged env vars
		assert.Contains(t, subDAG.Env, "ENV=production")
		assert.Contains(t, subDAG.Env, "API_KEY=secret123")
		assert.Contains(t, subDAG.Env, "SERVICE=worker")
	})

	t.Run("SingleDAGFileCompatibility", func(t *testing.T) {
		t.Parallel()

		// Single DAG file (no document separator)
		singleDAGContent := `steps:
  - name: step1
    command: echo "hello"
`
		tmpFile := createTempYAMLFile(t, singleDAGContent)

		// Load single DAG file
		dag, err := spec.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify it loads correctly without sub DAGs
		assert.Len(t, dag.Steps, 1)
		assert.Nil(t, dag.LocalDAGs) // No sub DAGs for single DAG file
	})

	// MultiDAG error cases
	multiDAGErrorTests := []struct {
		name        string
		content     string
		errContains string
	}{
		{
			name: "DuplicateSubDAGNames",
			content: `steps:
  - name: step1
    command: echo "main"

---
name: duplicate-name
steps:
  - name: step1
    command: echo "first"

---
name: duplicate-name
steps:
  - name: step1
    command: echo "second"
`,
			errContains: "duplicate DAG name",
		},
		{
			name: "SubDAGWithoutName",
			content: `steps:
  - name: step1
    command: echo "main"

---
steps:
  - name: step1
    command: echo "unnamed"
`,
			errContains: "must have a name",
		},
	}

	for _, tt := range multiDAGErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpFile := createTempYAMLFile(t, tt.content)
			_, err := spec.Load(context.Background(), tmpFile)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}

	t.Run("EmptyDocumentSeparator", func(t *testing.T) {
		t.Parallel()

		// TODO: The YAML parser has limitations with empty documents (---)
		// The behavior is inconsistent - sometimes it skips them, sometimes it errors.
		// For now, we test that it loads something, but the sub DAG after
		// the empty document may or may not be loaded.
		multiDAGContent := `steps:
  - name: step1
    command: echo "main"

---

---
name: child
steps:
  - name: step1
    command: echo "child"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// The behavior with empty documents is unpredictable
		_, err := spec.Load(context.Background(), tmpFile)
		if err != nil {
			// If it errors, it should be a decode error
			assert.Contains(t, err.Error(), "failed to decode document")
		}
	})

	t.Run("ComplexMultiDAGWithParameters", func(t *testing.T) {
		t.Parallel()

		// Complex multi-DAG with parameters
		// Using type: graph for sub-DAG that needs explicit dependencies
		multiDAGContent := `params:
  - ENVIRONMENT: dev
schedule: "0 2 * * *"
steps:
  - name: extract
    call: extract-module
    params: "SOURCE=customers TABLE=users"
  - name: transform
    call: transform-module

---
name: extract-module
type: graph
params:
  - SOURCE: default_source
  - TABLE: default_table
steps:
  - name: validate
    command: test -f data/${SOURCE}/${TABLE}
  - name: extract
    command: extract.py --source=${SOURCE} --table=${TABLE}
    depends: validate

---
name: transform-module
steps:
  - name: transform
    command: transform.py
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		dag, err := spec.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify main DAG
		assert.Len(t, dag.Schedule, 1)
		assert.Equal(t, "0 2 * * *", dag.Schedule[0].Expression)
		assert.Contains(t, dag.Params, "ENVIRONMENT=dev")

		// Verify sub DAG references and parameters
		assert.Equal(t, "extract-module", dag.Steps[0].SubDAG.Name)
		assert.Equal(t, `SOURCE="customers" TABLE="users"`, dag.Steps[0].SubDAG.Params)
		assert.Equal(t, "transform-module", dag.Steps[1].SubDAG.Name)

		// Verify extract-module sub DAG
		extractDAG := dag.LocalDAGs["extract-module"]
		require.NotNil(t, extractDAG)
		assert.Contains(t, extractDAG.Params, "SOURCE=default_source")
		assert.Contains(t, extractDAG.Params, "TABLE=default_table")
		assert.Len(t, extractDAG.Steps, 2)

		// Verify dependencies in sub DAG
		assert.Contains(t, extractDAG.Steps[1].Depends, "validate")
	})

	t.Run("WorkerSelector", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `description: Test DAG with worker selector
worker_selector:
  gpu: "true"
  memory: "64G"
steps:
  - name: gpu-task
    command: echo "Running on GPU worker"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)

		// Verify DAG loaded successfully
		assert.Equal(t, "Test DAG with worker selector", dag.Description)
		assert.Len(t, dag.Steps, 1)

		// Verify the step with GPU selector
		assert.NotNil(t, dag.WorkerSelector)
		assert.Equal(t, "true", dag.WorkerSelector["gpu"])
		assert.Equal(t, "64G", dag.WorkerSelector["memory"])
	})
}

func TestWithDefaultWorkingDir(t *testing.T) {
	t.Parallel()

	t.Run("DefaultUsedWhenNoFileContext", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory to use as default working dir
		tmpDir := t.TempDir()

		// Load from YAML data (no file context) with WithDefaultWorkingDir option
		dag, err := spec.LoadYAML(context.Background(), []byte(`steps:
  - name: test
    command: echo hello
`), spec.WithDefaultWorkingDir(tmpDir))
		require.NoError(t, err)

		// The WorkingDir should be set to the provided default value
		assert.Equal(t, tmpDir, dag.WorkingDir)
	})

	t.Run("DefaultTakesPrecedenceOverFileContext", func(t *testing.T) {
		t.Parallel()

		// Create a temporary directory for default
		defaultDir := t.TempDir()

		// Create a DAG file without explicit working_dir
		testDAG := createTempYAMLFile(t, `steps:
  - name: test
    command: echo hello
`)
		fileDir := filepath.Dir(testDAG)

		// First, verify that without the option, file's directory is used
		dagWithoutOption, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		assert.Equal(t, fileDir, dagWithoutOption.WorkingDir, "Without option, should use file's directory")

		// Now load with WithDefaultWorkingDir option
		// The default should take precedence over the file's directory
		dag, err := spec.Load(context.Background(), testDAG, spec.WithDefaultWorkingDir(defaultDir))
		require.NoError(t, err)

		// The WorkingDir should be the default, not the DAG file's directory
		assert.Equal(t, defaultDir, dag.WorkingDir)
		assert.NotEqual(t, fileDir, dag.WorkingDir, "Default should take precedence over file's directory")
	})

	t.Run("ExplicitWorkingDirTakesPrecedence", func(t *testing.T) {
		t.Parallel()

		// Create temporary directories
		explicitDir := t.TempDir()
		defaultDir := t.TempDir()

		// Create a DAG file with explicit working_dir
		testDAG := createTempYAMLFile(t, `working_dir: `+explicitDir+`
steps:
  - name: test
    command: echo hello
`)
		// Load with WithDefaultWorkingDir option (should be ignored since DAG has explicit working_dir)
		dag, err := spec.Load(context.Background(), testDAG, spec.WithDefaultWorkingDir(defaultDir))
		require.NoError(t, err)

		// The explicit working_dir from the DAG should take precedence
		assert.Equal(t, explicitDir, dag.WorkingDir)
	})
}

func TestLoadWithLoaderOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithDAGsDir", func(t *testing.T) {
		t.Parallel()

		// Create a DAGs directory
		dagsDir := t.TempDir()

		// Create a sub-DAG file
		subDAGPath := filepath.Join(dagsDir, "subdag.yaml")
		require.NoError(t, os.WriteFile(subDAGPath, []byte(`
steps:
  - name: sub-step
    command: echo sub
`), 0644))

		// Create main DAG that calls the sub-DAG
		mainDAG := createTempYAMLFile(t, `
steps:
  - name: main-step
    command: echo main
`)
		// Load with WithDAGsDir
		dag, err := spec.Load(context.Background(), mainDAG, spec.WithDAGsDir(dagsDir))
		require.NoError(t, err)
		require.NotNil(t, dag)
	})

	t.Run("WithAllowBuildErrors", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
steps:
  - name: test
    command: echo test
    depends:
      - nonexistent-step
`)
		// Without AllowBuildErrors, this would fail
		_, err := spec.Load(context.Background(), testDAG)
		require.Error(t, err)

		// With AllowBuildErrors, it should succeed but capture errors
		dag, err := spec.Load(context.Background(), testDAG, spec.WithAllowBuildErrors())
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.NotEmpty(t, dag.BuildErrors)
	})

	t.Run("SkipSchemaValidation", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
params:
  schema: "nonexistent-schema.json"
  values:
    foo: bar
steps:
  - name: test
    command: echo test
`)
		// Without SkipSchemaValidation, this would fail due to missing schema
		_, err := spec.Load(context.Background(), testDAG)
		require.Error(t, err)

		// With SkipSchemaValidation, it should succeed
		dag, err := spec.Load(context.Background(), testDAG, spec.SkipSchemaValidation())
		require.NoError(t, err)
		require.NotNil(t, dag)
	})

	t.Run("WithSkipBaseHandlers", func(t *testing.T) {
		t.Parallel()

		// Create base config with handlers
		baseDir := t.TempDir()
		baseConfig := filepath.Join(baseDir, "base.yaml")
		require.NoError(t, os.WriteFile(baseConfig, []byte(`
handler_on:
  success:
    command: echo base-success
`), 0644))

		testDAG := createTempYAMLFile(t, `
steps:
  - name: test
    command: echo test
`)
		// Load with base config but skip base handlers
		dag, err := spec.Load(context.Background(), testDAG,
			spec.WithBaseConfig(baseConfig),
			spec.WithSkipBaseHandlers())
		require.NoError(t, err)
		require.NotNil(t, dag)
		// The base success handler should not be present
		assert.Nil(t, dag.HandlerOn.Success)
	})

	t.Run("WithParamsAsList", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
params: KEY1 KEY2
steps:
  - name: test
    command: echo $KEY1 $KEY2
`)
		// Load with params as list
		dag, err := spec.Load(context.Background(), testDAG,
			spec.WithParams([]string{"KEY1=value1", "KEY2=value2"}))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Check that params were applied
		found := 0
		for _, p := range dag.Params {
			if p == "KEY1=value1" || p == "KEY2=value2" {
				found++
			}
		}
		assert.Equal(t, 2, found, "Both params should be applied")
	})
}

// TestLoadWithoutEval tests the WithoutEval loader option
// This test cannot be parallel because it uses t.Setenv
func TestLoadWithoutEval(t *testing.T) {
	t.Setenv("TEST_VAR", "should-not-expand")

	testDAG := createTempYAMLFile(t, `
env:
  - MY_VAR: "${TEST_VAR}"
steps:
  - name: test
    command: echo test
`)
	dag, err := spec.Load(context.Background(), testDAG, spec.WithoutEval())
	require.NoError(t, err)

	// With NoEval, environment variables should not be expanded
	assert.Contains(t, dag.Env, "MY_VAR=${TEST_VAR}")
}

func TestDefaults_LoadDAG(t *testing.T) {
	t.Parallel()

	t.Run("StepsInheritDefaults", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  retry_policy:
    limit: 3
    interval_sec: 5
  timeout_sec: 600
  mail_on_error: true

steps:
  - name: step1
    command: echo "hello"
  - name: step2
    command: echo "world"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		for _, step := range dag.Steps {
			assert.Equal(t, 3, step.RetryPolicy.Limit, "step %s should inherit retry limit", step.Name)
			assert.Equal(t, 5*time.Second, step.RetryPolicy.Interval, "step %s should inherit retry interval", step.Name)
			assert.Equal(t, 600*time.Second, step.Timeout, "step %s should inherit timeout", step.Name)
			assert.True(t, step.MailOnError, "step %s should inherit mail_on_error", step.Name)
		}
	})

	t.Run("StepOverridesDefault", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  retry_policy:
    limit: 3
    interval_sec: 5
  timeout_sec: 600

steps:
  - name: step1
    command: echo "inherits"
  - name: step2
    command: echo "overrides"
    retry_policy:
      limit: 10
      interval_sec: 30
    timeout_sec: 300
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// step1 inherits defaults
		assert.Equal(t, 3, dag.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 600*time.Second, dag.Steps[0].Timeout)

		// step2 overrides defaults
		assert.Equal(t, 10, dag.Steps[1].RetryPolicy.Limit)
		assert.Equal(t, 30*time.Second, dag.Steps[1].RetryPolicy.Interval)
		assert.Equal(t, 300*time.Second, dag.Steps[1].Timeout)
	})

	t.Run("AdditiveEnvAndPreconditions", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  env:
    - DEFAULT_VAR: default_value
  preconditions:
    - condition: "true"

steps:
  - name: step1
    command: echo "only defaults"
  - name: step2
    command: echo "both"
    env:
      - STEP_VAR: step_value
    preconditions:
      - condition: "test -d /tmp"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// step1 gets only defaults
		assert.Contains(t, dag.Steps[0].Env, "DEFAULT_VAR=default_value")
		require.Len(t, dag.Steps[0].Preconditions, 1)

		// step2 gets both defaults + step-level (additive)
		assert.Contains(t, dag.Steps[1].Env, "DEFAULT_VAR=default_value")
		assert.Contains(t, dag.Steps[1].Env, "STEP_VAR=step_value")
		require.Len(t, dag.Steps[1].Preconditions, 2)
	})

	t.Run("HandlersInheritDefaults", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  timeout_sec: 300

handler_on:
  failure:
    command: echo "failure handler"
  exit:
    command: echo "exit handler"
    timeout_sec: 60

steps:
  - name: step1
    command: echo "test"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)

		// failure handler inherits default timeout
		require.NotNil(t, dag.HandlerOn.Failure)
		assert.Equal(t, 300*time.Second, dag.HandlerOn.Failure.Timeout)

		// exit handler overrides default timeout
		require.NotNil(t, dag.HandlerOn.Exit)
		assert.Equal(t, 60*time.Second, dag.HandlerOn.Exit.Timeout)
	})

	t.Run("BaseConfigDefaults", func(t *testing.T) {
		t.Parallel()

		base := createTempYAMLFile(t, `
defaults:
  timeout_sec: 300
`)
		child := createTempYAMLFile(t, `
defaults:
  timeout_sec: 600

steps:
  - name: step1
    command: echo "test"
`)
		dag, err := spec.Load(context.Background(), child, spec.WithBaseConfig(base))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// DAG-level defaults should override base config defaults
		assert.Equal(t, 600*time.Second, dag.Steps[0].Timeout)
	})

	t.Run("UnknownKeyInDefaults", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  timeout_sec: 600
  unknown_field: value

steps:
  - name: step1
    command: echo "test"
`)
		_, err := spec.Load(context.Background(), testDAG)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "defaults")
	})

	t.Run("ContinueOnDefault", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  continue_on: failed

steps:
  - name: step1
    command: echo "test"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.True(t, dag.Steps[0].ContinueOn.Failure)
	})

	t.Run("SignalOnStopDefault", func(t *testing.T) {
		t.Parallel()

		testDAG := createTempYAMLFile(t, `
defaults:
  signal_on_stop: SIGTERM

steps:
  - name: step1
    command: echo "test"
`)
		dag, err := spec.Load(context.Background(), testDAG)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "SIGTERM", dag.Steps[0].SignalOnStop)
	})
}
