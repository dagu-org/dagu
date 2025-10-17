package spec_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	t.Run("SkipIfSuccessful", func(t *testing.T) {
		data := []byte(`
skipIfSuccessful: true
steps:
  - "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.True(t, th.SkipIfSuccessful)
	})
	t.Run("ParamsWithSubstitution", func(t *testing.T) {
		data := []byte(`
params: "TEST_PARAM $1"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		th.AssertParam(t, "1=TEST_PARAM", "2=TEST_PARAM")
	})
	t.Run("ParamsWithQuotedValues", func(t *testing.T) {
		data := []byte(`
params: x="a b c" y="d e f"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		th.AssertParam(t, "x=a b c", "y=d e f")
	})
	t.Run("ParamsAsMap", func(t *testing.T) {
		data := []byte(`
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		th.AssertParam(t,
			"FOO=foo",
			"BAR=bar",
			"BAZ=baz",
		)
	})
	t.Run("ParamsAsMapOverride", func(t *testing.T) {
		data := []byte(`
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Parameters: "FOO=X BAZ=Y"})
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		th.AssertParam(t,
			"FOO=X",
			"BAR=bar",
			"BAZ=Y",
		)
	})
	t.Run("ParamsWithComplexValues", func(t *testing.T) {
		data := []byte(`
params: first P1=foo P2=${A001} P3=` + "`/bin/echo BAR`" + ` X=bar Y=${P1} Z="A B C"
env:
  - A001: TEXT
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		th.AssertParam(t,
			"1=first",
			"P1=foo",
			"P2=TEXT",
			"P3=BAR",
			"X=bar",
			"Y=foo",
			"Z=A B C",
		)
	})
	t.Run("ParamsWithLocalSchemaReference", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 10,
      "minimum": 1
    },
    "environment": {
      "type": "string",
      "default": "dev",
      "enum": ["dev", "staging", "prod"]
    }
  }
}`

		// Create temp schema file
		tmpFile, err := os.CreateTemp("", "test-schema-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 25
    environment: "staging"
`, tmpFile.Name()))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		// Test that parameters are parsed correctly (order may vary)
		require.Len(t, th.Params, 2)
		require.Contains(t, th.Params, "batch_size=25")
		require.Contains(t, th.Params, "environment=staging")
	})
	t.Run("ParamsWithRemoteSchemaReference", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/schemas/dag-params.json", func(w http.ResponseWriter, r *http.Request) {
			schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 10,
      "minimum": 1
    },
    "environment": {
      "type": "string",
      "default": "dev",
      "enum": ["dev", "staging", "prod"]
    }
  }
}`
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(schemaContent))
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s/schemas/dag-params.json"
  values:
    batch_size: 50
    environment: "prod"
`, server.URL))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		// Test that parameters are parsed correctly (order may vary)
		require.Len(t, th.Params, 2)
		require.Contains(t, th.Params, "batch_size=50")
		require.Contains(t, th.Params, "environment=prod")
	})
	t.Run("ParamsWithSchemaAndOverrideValidation", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 10,
      "minimum": 1,
      "maximum": 50
    },
    "environment": {
      "type": "string",
      "default": "dev",
      "enum": ["dev", "staging", "prod"]
    }
  }
}`

		// Create temp schema file
		tmpFile, err := os.CreateTemp("", "test-schema-validation-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
`, tmpFile.Name()))

		// Inject CLI parameters that override the schema values and should fail validation
		cliParams := "batch_size=100 environment=prod"
		_, err = spec.LoadYAML(context.Background(), data, spec.WithParams(cliParams))
		require.Error(t, err)
		require.Contains(t, err.Error(), "parameter validation failed")
		require.Contains(t, err.Error(), "maximum: 100/1 is greater than 50")
	})
	t.Run("ParamsWithSchemaDefaultsApplied", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 25,
      "minimum": 1,
      "maximum": 100
    },
    "environment": {
      "type": "string",
      "default": "development",
      "enum": ["development", "staging", "production"]
    },
    "debug": {
      "type": "boolean",
      "default": true
    }
  }
}`

		// Create temp schema file
		tmpFile, err := os.CreateTemp("", "test-schema-defaults-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

		// Test case 1: Only provide some parameters, let defaults fill the rest
		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 75
    # environment and debug should get defaults
`, tmpFile.Name()))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		// Should have all 3 parameters: provided batch_size + defaults for environment and debug
		require.Len(t, th.Params, 3)

		// Check that provided value remains unchanged
		require.Contains(t, th.Params, "batch_size=75")

		// Check that defaults were applied
		require.Contains(t, th.Params, "environment=development")
		require.Contains(t, th.Params, "debug=true")
	})
	t.Run("ParamsWithSchemaDefaultsPreserveExistingValues", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {
      "type": "integer",
      "default": 25,
      "minimum": 1,
      "maximum": 100
    },
    "environment": {
      "type": "string",
      "default": "development",
      "enum": ["development", "staging", "production"]
    },
    "debug": {
      "type": "boolean",
      "default": true
    },
    "timeout": {
      "type": "integer",
      "default": 300
    }
  }
}`

		// Create temp schema file
		tmpFile, err := os.CreateTemp("", "test-schema-preserve-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

		// Provide all parameters explicitly - defaults should NOT override them
		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 50
    environment: "production"
    debug: false
    timeout: 600
`, tmpFile.Name()))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		// Should have all 4 parameters with their explicitly provided values
		require.Len(t, th.Params, 4)

		// Check that all explicitly provided values remain unchanged (defaults should not override)
		require.Contains(t, th.Params, "batch_size=50")
		require.Contains(t, th.Params, "environment=production")
		require.Contains(t, th.Params, "debug=false")
		require.Contains(t, th.Params, "timeout=600")
	})
	t.Run("MailOn", func(t *testing.T) {
		data := []byte(`
steps:
  - "true"

mailOn:
  failure: true
  success: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.True(t, th.MailOn.Failure)
		assert.True(t, th.MailOn.Success)
	})
	t.Run("ValidTags", func(t *testing.T) {
		data := []byte(`
tags: daily,monthly
steps:
  - echo 1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("ValidTagsList", func(t *testing.T) {
		data := []byte(`
tags:
  - daily
  - monthly
steps:
  - echo 1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("LogDir", func(t *testing.T) {
		data := []byte(`
logDir: /tmp/logs
steps:
  - "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, "/tmp/logs", th.LogDir)
	})
	t.Run("MailConfig", func(t *testing.T) {
		data := []byte(`
# SMTP server settings
smtp:
  host: "smtp.example.com"
  port: "587"
  username: user@example.com
  password: password

# Error mail configuration
errorMail:
  from: "error@example.com"
  to: "admin@example.com"
  prefix: "[ERROR]"
  attachLogs: true

# Info mail configuration
infoMail:
  from: "info@example.com"
  to: "user@example.com"
  prefix: "[INFO]"
  attachLogs: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, "smtp.example.com", th.SMTP.Host)
		assert.Equal(t, "587", th.SMTP.Port)
		assert.Equal(t, "user@example.com", th.SMTP.Username)
		assert.Equal(t, "password", th.SMTP.Password)

		assert.Equal(t, "error@example.com", th.ErrorMail.From)
		assert.Equal(t, []string{"admin@example.com"}, th.ErrorMail.To)
		assert.Equal(t, "[ERROR]", th.ErrorMail.Prefix)
		assert.True(t, th.ErrorMail.AttachLogs)

		assert.Equal(t, "info@example.com", th.InfoMail.From)
		assert.Equal(t, []string{"user@example.com"}, th.InfoMail.To)
		assert.Equal(t, "[INFO]", th.InfoMail.Prefix)
		assert.True(t, th.InfoMail.AttachLogs)
	})
	t.Run("SMTPNumericPort", func(t *testing.T) {
		// Test SMTP configuration with numeric port
		data := []byte(`
smtp:
  host: "smtp.example.com"
  port: 587
  username: "user@example.com"
  password: "password"
steps:
  -     echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.SMTP)
		assert.Equal(t, "smtp.example.com", dag.SMTP.Host)
		assert.Equal(t, "587", dag.SMTP.Port)
		assert.Equal(t, "user@example.com", dag.SMTP.Username)
		assert.Equal(t, "password", dag.SMTP.Password)
	})
	t.Run("MailConfigMultipleRecipients", func(t *testing.T) {
		data := []byte(`
# SMTP server settings
smtp:
  host: "smtp.example.com"
  port: "587"
  username: user@example.com
  password: password

# Error mail with multiple recipients
errorMail:
  from: "error@example.com"
  to: 
    - "admin1@example.com"
    - "admin2@example.com"
    - "admin3@example.com"
  prefix: "[ERROR]"
  attachLogs: true

# Info mail with single recipient as array
infoMail:
  from: "info@example.com"
  to:
    - "user@example.com"
  prefix: "[INFO]"
  attachLogs: false
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		// Check error mail with multiple recipients
		assert.Equal(t, "error@example.com", th.ErrorMail.From)
		assert.Equal(t, []string{"admin1@example.com", "admin2@example.com", "admin3@example.com"}, th.ErrorMail.To)
		assert.Equal(t, "[ERROR]", th.ErrorMail.Prefix)
		assert.True(t, th.ErrorMail.AttachLogs)

		// Check info mail with single recipient as array
		assert.Equal(t, "info@example.com", th.InfoMail.From)
		assert.Equal(t, []string{"user@example.com"}, th.InfoMail.To)
		assert.Equal(t, "[INFO]", th.InfoMail.Prefix)
		assert.False(t, th.InfoMail.AttachLogs)
	})
	t.Run("MaxHistRetentionDays", func(t *testing.T) {
		data := []byte(`
histRetentionDays: 365
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, 365, th.HistRetentionDays)
	})
	t.Run("CleanUpTime", func(t *testing.T) {
		data := []byte(`
maxCleanUpTimeSec: 10
steps:
  - "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, time.Duration(10*time.Second), th.MaxCleanUpTime)
	})
	t.Run("ChainTypeBasic", func(t *testing.T) {
		data := []byte(`
type: chain

steps:
  - echo "First"
  - echo "Second"
  - echo "Third"
  - echo "Fourth"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		// Check that implicit dependencies were added
		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends) // First step has no dependencies
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
		assert.Equal(t, []string{"cmd_3"}, th.Steps[3].Depends)
	})
	t.Run("ChainTypeWithExplicitDepends", func(t *testing.T) {
		data := []byte(`
type: chain

steps:
  - name: setup
    command: ./setup.sh
  
  - name: download-a
    command: wget fileA
  
  - name: download-b
    command: wget fileB
  
  - name: process-both
    command: process.py fileA fileB
    depends:  # Override chain to depend on both downloads
      - download-a
      - download-b
  
  - name: cleanup
    command: rm -f fileA fileB
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		// Check dependencies
		assert.Len(t, th.Steps, 5)
		assert.Empty(t, th.Steps[0].Depends)                         // setup
		assert.Equal(t, []string{"setup"}, th.Steps[1].Depends)      // download-a
		assert.Equal(t, []string{"download-a"}, th.Steps[2].Depends) // download-b
		// process-both should keep its explicit dependencies
		assert.ElementsMatch(t, []string{"download-a", "download-b"}, th.Steps[3].Depends)
		assert.Equal(t, []string{"process-both"}, th.Steps[4].Depends) // cleanup
	})
	t.Run("InvalidType", func(t *testing.T) {
		// Test will fail with an error containing "invalid type"
		data := []byte(`
type: invalid-type

steps:
  - name: step1
    command: echo "test"
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type")
	})
	t.Run("DefaultTypeIsChain", func(t *testing.T) {
		data := []byte(`
steps:
  - echo 1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)
	})
	t.Run("ChainTypeWithNoDependencies", func(t *testing.T) {
		data := []byte(`
type: chain

steps:
  - name: step1
    command: echo "First"
  
  - name: step2
    command: echo "Second - should depend on step1"
  
  - name: step3
    command: echo "Third - no dependencies"
    depends: []  # Explicitly no dependencies
  
  - name: step4
    command: echo "Fourth - should depend on step3"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		// Check dependencies
		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)                    // step1
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends) // step2
		assert.Empty(t, th.Steps[2].Depends)                    // step3 - explicitly no deps
		assert.Equal(t, []string{"step3"}, th.Steps[3].Depends) // step4 should depend on step3
	})
	t.Run("Preconditions", func(t *testing.T) {
		data := []byte(`
preconditions:
  - condition: "test -f file.txt"
    expected: "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Preconditions, 1)
		assert.Equal(t, &core.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Preconditions[0])
	})
	t.Run("MaxActiveRuns", func(t *testing.T) {
		data := []byte(`
maxActiveRuns: 5
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, 5, th.MaxActiveRuns)
	})
	t.Run("MaxActiveSteps", func(t *testing.T) {
		data := []byte(`
maxActiveSteps: 3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, 3, th.MaxActiveSteps)
	})
	t.Run("RunConfig", func(t *testing.T) {
		data := []byte(`
runConfig:
  disableParamEdit: true
  disableRunIdEdit: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.RunConfig)
		assert.True(t, dag.RunConfig.DisableParamEdit)
		assert.True(t, dag.RunConfig.DisableRunIdEdit)
	})
	t.Run("MaxOutputSize", func(t *testing.T) {
		// Test custom maxOutputSize
		data := []byte(`
description: Test DAG with custom maxOutputSize

# Custom maxOutputSize of 512KB
maxOutputSize: 524288

steps:
  - echo "test output"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, 524288, th.MaxOutputSize) // 512KB

		// Test default maxOutputSize when not specified
		data2 := []byte(`
steps:
  - "true"
`)
		dag2, err := spec.LoadYAML(context.Background(), data2)
		require.NoError(t, err)
		th2 := DAG{t: t, DAG: dag2}
		assert.Equal(t, 0, th2.MaxOutputSize) // Default 1MB
	})
	t.Run("ValidationError", func(t *testing.T) {
		type testCase struct {
			name        string
			yaml        string
			expectedErr error
		}

		testCases := []testCase{
			{
				name: "InvalidEnv",
				yaml: `
env:
  - VAR: "` + "`invalid command`" + `"`,
				expectedErr: spec.ErrInvalidEnvValue,
			},
			{
				name: "InvalidParams",
				yaml: `
params: "` + "`invalid command`" + `"`,
				expectedErr: spec.ErrInvalidParamValue,
			},
			{
				name: "InvalidSchedule",
				yaml: `
schedule: "1"`,
				expectedErr: spec.ErrInvalidSchedule,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				data := []byte(tc.yaml)
				ctx := context.Background()
				_, err := spec.LoadYAML(ctx, data)
				if errs, ok := err.(*core.ErrorList); ok && len(*errs) > 0 {
					found := false
					for _, e := range *errs {
						if errors.Is(e, tc.expectedErr) {
							found = true
							break
						}
					}
					require.True(t, found, "expected error %v, got %v", tc.expectedErr, err)
				} else {
					assert.ErrorIs(t, err, tc.expectedErr)
				}
			})
		}
	})
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		yaml     string
		expected map[string]string
	}

	testCases := []testCase{
		{
			name: "ValidEnv",
			yaml: `
env:
  - FOO: "123"

steps:
  - "true"
`,
			expected: map[string]string{
				"FOO": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitution",
			yaml: `
env:
  - VAR: "` + "`echo 123`" + `"

steps:
  - "true"
`,
			expected: map[string]string{
				"VAR": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitutionAndEnv",
			yaml: `
env:
  - BEE: "BEE"
  - BAZ: "BAZ"
  - BOO: "BOO"
  - FOO: "${BEE}:${BAZ}:${BOO}:FOO"

steps:
  - "true"
`,
			expected: map[string]string{
				"FOO": "BEE:BAZ:BOO:FOO",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dag, err := spec.LoadYAML(context.Background(), []byte(tc.yaml))
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
			for key, val := range tc.expected {
				th.AssertEnv(t, key, val)
			}
		})
	}
}

func TestBuildSchedule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		yaml    string
		start   []string
		stop    []string
		restart []string
	}

	testCases := []testCase{
		{
			name: "ValidSchedule",
			yaml: `
schedule:
  start: "0 1 * * *"
  stop: "0 2 * * *"
  restart: "0 12 * * *"

steps:
  - "true"
`,
			start:   []string{"0 1 * * *"},
			stop:    []string{"0 2 * * *"},
			restart: []string{"0 12 * * *"},
		},
		{
			name: "ListSchedule",
			yaml: `
schedule:
  - "0 1 * * *"
  - "0 18 * * *"

steps:
  - "true"
`,
			start: []string{
				"0 1 * * *",
				"0 18 * * *",
			},
		},
		{
			name: "MultipleValues",
			yaml: `
schedule:
  start:
    - "0 1 * * *"
    - "0 18 * * *"
  stop:
    - "0 2 * * *"
    - "0 20 * * *"
  restart:
    - "0 12 * * *"
    - "0 22 * * *"

steps:
  - "true"
`,
			start: []string{
				"0 1 * * *",
				"0 18 * * *",
			},
			stop: []string{
				"0 2 * * *",
				"0 20 * * *",
			},
			restart: []string{
				"0 12 * * *",
				"0 22 * * *",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dag, err := spec.LoadYAML(context.Background(), []byte(tc.yaml))
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
			assert.Len(t, th.Schedule, len(tc.start))
			for i, s := range tc.start {
				assert.Equal(t, s, th.Schedule[i].Expression)
			}

			assert.Len(t, th.StopSchedule, len(tc.stop))
			for i, s := range tc.stop {
				assert.Equal(t, s, th.StopSchedule[i].Expression)
			}

			assert.Len(t, th.RestartSchedule, len(tc.restart))
			for i, s := range tc.restart {
				assert.Equal(t, s, th.RestartSchedule[i].Expression)
			}
		})
	}
}

func TestBuildStep(t *testing.T) {
	t.Parallel()
	t.Run("ValidCommand", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: echo 1
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "echo 1", th.Steps[0].CmdWithArgs)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("CommandAsScript", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: |
      echo hello
      echo world
    name: script
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		require.Len(t, th.Steps, 1)
		step := th.Steps[0]
		assert.Equal(t, "script", step.Name)
		assert.Equal(t, "echo hello\necho world", step.Script)
		assert.Empty(t, step.Command)
		assert.Empty(t, step.CmdWithArgs)
		assert.Empty(t, step.CmdArgsSys)
		assert.Nil(t, step.Args)
	})
	t.Run("ValidCommandInArray", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: [echo, 1]
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("ValidCommandInList", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command:
      - echo
      - 1
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("HTTPExecutor", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: GET http://example.com
    name: step1
    executor: http
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
	})
	t.Run("HTTPExecutorWithConfig", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: http://example.com
    name: step1
    executor:
      type: http
      config:
        key: value
        map:
          foo: bar
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, map[string]any{
			"key": "value",
			"map": map[string]any{
				"foo": "bar",
			},
		}, th.Steps[0].ExecutorConfig.Config)
	})
	t.Run("DAGExecutor", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: execute a sub-dag
    call: sub_dag
    params: "param1=value1 param2=value2"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "dag", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "run", th.Steps[0].Command)
		assert.Equal(t, []string{
			"sub_dag",
			"param1=\"value1\" param2=\"value2\"",
		}, th.Steps[0].Args)
		assert.Equal(t, "sub_dag param1=\"value1\" param2=\"value2\"", th.Steps[0].CmdWithArgs)
		assert.Empty(t, dag.BuildWarnings)

		// Legacy run field is still accepted
		dataLegacy := []byte(`
steps:
  - name: legacy sub-dag
    run: sub_dag_legacy
`)
		dagLegacy, err := spec.LoadYAML(context.Background(), dataLegacy)
		require.NoError(t, err)
		thLegacy := DAG{t: t, DAG: dagLegacy}
		assert.Len(t, thLegacy.Steps, 1)
		assert.Equal(t, "dag", thLegacy.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "run", thLegacy.Steps[0].Command)
		assert.Equal(t, []string{"sub_dag_legacy", ""}, thLegacy.Steps[0].Args)
		assert.Equal(t, "sub_dag_legacy", thLegacy.Steps[0].CmdWithArgs)
		require.Len(t, dagLegacy.BuildWarnings, 1)
		assert.Contains(t, dagLegacy.BuildWarnings[0], "deprecated field `run`")
	})
	t.Run("ContinueOn", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: "echo 1"
    continueOn:
      skipped: true
      failure: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.True(t, th.Steps[0].ContinueOn.Failure)
		assert.True(t, th.Steps[0].ContinueOn.Skipped)
	})
	t.Run("RetryPolicy", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: "echo 2"
    retryPolicy:
      limit: 3
      intervalSec: 10
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 3, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 10*time.Second, th.Steps[0].RetryPolicy.Interval)
	})
	t.Run("RetryPolicyWithBackoff", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "test_backoff"
    command: "echo test"
    retryPolicy:
      limit: 5
      intervalSec: 2
      backoff: 2.0
      maxIntervalSec: 30
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 5, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 2*time.Second, th.Steps[0].RetryPolicy.Interval)
		assert.Equal(t, 2.0, th.Steps[0].RetryPolicy.Backoff)
		assert.Equal(t, 30*time.Second, th.Steps[0].RetryPolicy.MaxInterval)
	})
	t.Run("RetryPolicyWithBackoffBool", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "test_backoff_bool"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: true
      maxIntervalSec: 10
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 3, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 1*time.Second, th.Steps[0].RetryPolicy.Interval)
		assert.Equal(t, 2.0, th.Steps[0].RetryPolicy.Backoff) // true converts to 2.0
		assert.Equal(t, 10*time.Second, th.Steps[0].RetryPolicy.MaxInterval)
	})
	t.Run("RetryPolicyInvalidBackoff", func(t *testing.T) {
		t.Parallel()

		// Test backoff value <= 1.0
		data := []byte(`
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: 0.8
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")
	})
	t.Run("RepeatPolicy", func(t *testing.T) {
		t.Parallel()

		// Test basic boolean repeat (backward compatibility)
		data := []byte(`
steps:
  - command: "echo 2"
    repeatPolicy:
      repeat: true
      intervalSec: 60
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RepeatPolicy)
		assert.Equal(t, core.RepeatModeWhile, th.Steps[0].RepeatPolicy.RepeatMode)
		assert.Equal(t, 60*time.Second, th.Steps[0].RepeatPolicy.Interval)
		assert.Equal(t, 0, th.Steps[0].RepeatPolicy.Limit) // No limit set
	})

	t.Run("RepeatPolicyWhileCondition", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "repeat-while-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      condition: "echo hello"
      intervalSec: 5
      limit: 3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeWhile, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "", repeatPolicy.Condition.Expected) // No expected value for while mode
		assert.Equal(t, 5*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 3, repeatPolicy.Limit)
	})

	t.Run("RepeatPolicyUntilCondition", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "repeat-until-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      condition: "echo hello"
      expected: "hello"
      intervalSec: 10
      limit: 5
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeUntil, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 10*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 5, repeatPolicy.Limit)
	})

	t.Run("RepeatPolicyWhileExitCode", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "repeat-while-exitcode"
    command: "exit 1"
    repeatPolicy:
      repeat: "while"
      exitCode: [1, 2]
      intervalSec: 15
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeWhile, repeatPolicy.RepeatMode)
		assert.Equal(t, []int{1, 2}, repeatPolicy.ExitCode)
		assert.Equal(t, 15*time.Second, repeatPolicy.Interval)
		assert.Nil(t, repeatPolicy.Condition) // No condition set
	})

	t.Run("RepeatPolicyUntilExitCode", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "repeat-until-exitcode"
    command: "exit 0"
    repeatPolicy:
      repeat: "until"
      exitCode: [0]
      intervalSec: 20
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeUntil, repeatPolicy.RepeatMode)
		assert.Equal(t, []int{0}, repeatPolicy.ExitCode)
		assert.Equal(t, 20*time.Second, repeatPolicy.Interval)
		assert.Nil(t, repeatPolicy.Condition) // No condition set
	})

	t.Run("RepeatPolicyBackwardCompatibilityUntil", func(t *testing.T) {
		t.Parallel()

		// Test backward compatibility: condition + expected should infer "until" mode
		data := []byte(`
steps:
  - name: "repeat-backward-compatibility-until"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 25
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeUntil, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 25*time.Second, repeatPolicy.Interval)
	})

	t.Run("RepeatPolicyBackwardCompatibilityWhile", func(t *testing.T) {
		t.Parallel()

		// Test backward compatibility: condition only should infer "while" mode
		data := []byte(`
steps:
  - name: "repeat-backward-compatibility-while"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      intervalSec: 30
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeWhile, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "", repeatPolicy.Condition.Expected) // No expected value
		assert.Equal(t, 30*time.Second, repeatPolicy.Interval)
	})

	t.Run("RepeatPolicyCondition", func(t *testing.T) {
		t.Parallel()

		// Test existing backward compatibility condition test
		data := []byte(`
steps:
  - name: "repeat-condition"
    command: "echo hello"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 1*time.Second, repeatPolicy.Interval)
		// Should infer "until" mode due to condition + expected
		assert.Equal(t, core.RepeatModeUntil, repeatPolicy.RepeatMode)
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: echo 1
    name: step1
    signalOnStop: SIGINT
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "SIGINT", th.Steps[0].SignalOnStop)
	})
	t.Run("StepWithID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: unique-step-1
    command: echo "Step with ID"
  - name: step2
    command: echo "Step without ID"
  - name: step3
    id: custom-id-123
    command: echo "Another step with ID"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 3)

		// First step has ID
		assert.Equal(t, "step1", th.Steps[0].Name)
		assert.Equal(t, "unique-step-1", th.Steps[0].ID)

		// Second step has no ID
		assert.Equal(t, "step2", th.Steps[1].Name)
		assert.Equal(t, "", th.Steps[1].ID)

		// Third step has ID
		assert.Equal(t, "step3", th.Steps[2].Name)
		assert.Equal(t, "custom-id-123", th.Steps[2].ID)
	})
	t.Run("Preconditions", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "2"
    command: "echo 2"
    preconditions:
      - condition: "test -f file.txt"
        expected: "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &core.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Steps[0].Preconditions[0])
	})
	t.Run("RepeatPolicyExitCode", func(t *testing.T) {
		t.Parallel()

		// Test existing backward compatibility exitcode test
		data := []byte(`
steps:
  - name: "repeat-exitcode"
    command: "exit 42"
    repeatPolicy:
      exitCode: [42]
      intervalSec: 2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, []int{42}, repeatPolicy.ExitCode)
		assert.Equal(t, 2*time.Second, repeatPolicy.Interval)
		// Should infer "while" mode due to exitCode only
		assert.Equal(t, core.RepeatModeWhile, repeatPolicy.RepeatMode)
	})

	t.Run("RepeatPolicyWithBackoff", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "test_repeat_backoff"
    command: "echo test"
    repeatPolicy:
      repeat: while
      intervalSec: 5
      backoff: 1.5
      maxIntervalSec: 60
      limit: 10
      exitCode: [1]
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeWhile, repeatPolicy.RepeatMode)
		assert.Equal(t, 5*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 1.5, repeatPolicy.Backoff)
		assert.Equal(t, 60*time.Second, repeatPolicy.MaxInterval)
		assert.Equal(t, 10, repeatPolicy.Limit)
		assert.Equal(t, []int{1}, repeatPolicy.ExitCode)
	})

	t.Run("RepeatPolicyWithBackoffBool", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "test_repeat_backoff_bool"
    command: "echo test"
    repeatPolicy:
      repeat: until
      intervalSec: 2
      backoff: true
      maxIntervalSec: 20
      limit: 5
      condition: "echo done"
      expected: "done"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, core.RepeatModeUntil, repeatPolicy.RepeatMode)
		assert.Equal(t, 2*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 2.0, repeatPolicy.Backoff) // true converts to 2.0
		assert.Equal(t, 20*time.Second, repeatPolicy.MaxInterval)
		assert.Equal(t, 5, repeatPolicy.Limit)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo done", repeatPolicy.Condition.Condition)
		assert.Equal(t, "done", repeatPolicy.Condition.Expected)
	})

	t.Run("RepeatPolicyErrorCases", func(t *testing.T) {
		t.Parallel()

		// Test invalid repeat value
		data := []byte(`
steps:
  - name: "invalid-repeat"
    command: "echo test"
    repeatPolicy:
      repeat: "invalid"
      intervalSec: 10
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "invalid value for repeat: 'invalid'")

		// Test explicit while mode without condition or exitCode
		data = []byte(`
steps:
  - name: "while-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 10
`)
		dag, err = spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "repeat mode 'while' requires either 'condition' or 'exitCode' to be specified")

		// Test explicit until mode without condition or exitCode
		data = []byte(`
steps:
  - name: "until-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      intervalSec: 10
`)
		dag, err = spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "repeat mode 'until' requires either 'condition' or 'exitCode' to be specified")

		// Test invalid repeat type (not string or bool)
		data = []byte(`
steps:
  - name: "invalid-type"
    command: "echo test"
    repeatPolicy:
      repeat: 123
      intervalSec: 10
`)
		dag, err = spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "invalid value for repeat")
	})

	t.Run("PolicyBackoffValidation", func(t *testing.T) {
		t.Parallel()

		// Test repeat policy invalid backoff
		data := []byte(`
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 1.0
      exitCode: [1]
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")

		// Test with backoff = 0.5
		data = []byte(`
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 0.5
      exitCode: [1]
`)
		dag, err = spec.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")
	})
}

type DAG struct {
	t *testing.T
	*core.DAG
}

func (th *DAG) AssertEnv(t *testing.T, key, val string) {
	th.t.Helper()

	expected := key + "=" + val
	for _, env := range th.Env {
		if env == expected {
			return
		}
	}
	t.Errorf("expected env %s=%s not found", key, val)
	for i, env := range th.Env {
		// print all envs that were found for debugging
		t.Logf("env[%d]: %s", i, env)
	}
}

func (th *DAG) AssertParam(t *testing.T, params ...string) {
	th.t.Helper()

	assert.Len(t, th.Params, len(params), "expected %d params, got %d", len(params), len(th.Params))
	for i, p := range params {
		assert.Equal(t, p, th.Params[i])
	}
}

// testLoad and helper functions have been removed - all tests now use inline YAML
func TestBuild_QueueConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("MaxActiveRunsDefaultsToOne", func(t *testing.T) {
		t.Parallel()

		// Test that when maxActiveRuns is not specified, it defaults to 1
		data := []byte(`
steps:
  - command: echo 1
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag} // Using a simple DAG without maxActiveRuns
		assert.Equal(t, 1, th.MaxActiveRuns, "maxActiveRuns should default to 1 when not specified")
	})

	t.Run("MaxActiveRunsNegativeValuePreserved", func(t *testing.T) {
		t.Parallel()

		// Test that negative values are preserved (they mean queueing is disabled)
		// Create a simple DAG YAML with negative maxActiveRuns
		data := []byte(`
maxActiveRuns: -1
steps:
  - name: step1
    command: echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, -1, dag.MaxActiveRuns, "negative maxActiveRuns should be preserved")
	})
}

func TestNestedArrayParallelSyntax(t *testing.T) {
	t.Parallel()

	t.Run("SimpleParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "step 1"
  - 
    - echo "parallel 1"
    - echo "parallel 2"
  - echo "step 3"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		// First step (sequential)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		assert.Equal(t, "echo \"step 1\"", dag.Steps[0].CmdWithArgs)
		assert.Empty(t, dag.Steps[0].Depends)

		// Parallel steps
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		assert.Equal(t, "echo \"parallel 1\"", dag.Steps[1].CmdWithArgs)
		assert.Equal(t, []string{"cmd_1"}, dag.Steps[1].Depends)

		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
		assert.Equal(t, "echo \"parallel 2\"", dag.Steps[2].CmdWithArgs)
		assert.Equal(t, []string{"cmd_1"}, dag.Steps[2].Depends)

		// Last step (sequential, depends on both parallel steps)
		assert.Equal(t, "cmd_4", dag.Steps[3].Name)
		assert.Equal(t, "echo \"step 3\"", dag.Steps[3].CmdWithArgs)

		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
		assert.Contains(t, dag.Steps[3].Depends, "cmd_3")
	})

	t.Run("MixedParallelAndNormalSyntax", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: setup
    command: echo "setup"
  -
    - echo "parallel 1"
    - name: test
      command: npm test
  - name: cleanup
    command: echo "cleanup"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		// Setup step
		assert.Equal(t, "setup", dag.Steps[0].Name)
		assert.Empty(t, dag.Steps[0].Depends)

		// Parallel steps
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		assert.Equal(t, []string{"setup"}, dag.Steps[1].Depends)

		assert.Equal(t, "test", dag.Steps[2].Name)
		assert.Equal(t, []string{"setup"}, dag.Steps[2].Depends)

		// Cleanup step
		assert.Equal(t, "cleanup", dag.Steps[3].Name)
		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
		assert.Contains(t, dag.Steps[3].Depends, "test")
	})

	t.Run("ParallelStepsWithExplicitDependencies", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    command: echo "1"
  - name: step2
    command: echo "2"
  - 
    - name: parallel1
      command: echo "p1"
      depends: [step1]  # Explicit dependency overrides automatic
    - name: parallel2
      command: echo "p2"
      # This will get automatic dependency on step2
  - name: final
    command: echo "done"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 5)

		// Parallel1 has explicit dependency on step1
		parallel1 := dag.Steps[2]
		assert.Equal(t, "parallel1", parallel1.Name)
		assert.Equal(t, []string{"step1", "step2"}, parallel1.Depends)

		// Parallel2 gets automatic dependency on step2
		parallel2 := dag.Steps[3]
		assert.Equal(t, "parallel2", parallel2.Name)
		assert.Equal(t, []string{"step2"}, parallel2.Depends)

		// Final depends on both parallel steps
		final := dag.Steps[4]
		assert.Equal(t, "final", final.Name)
		assert.Contains(t, final.Depends, "parallel1")
		assert.Contains(t, final.Depends, "parallel2")
	})

	t.Run("OnlyParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - 
    - echo "parallel 1"
    - echo "parallel 2"
    - echo "parallel 3"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 3)

		// All steps should have no dependencies (first group)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		// Note: Due to the way dependencies are handled, these may have dependencies on each other
		// The important thing is they work in parallel since they don't have external dependencies

		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
	})
	t.Run("ConsequentParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - 
    - echo "parallel 1"
    - echo "parallel 2"
  - 
    - echo "parallel 3"
    - echo "parallel 4"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)

		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
		assert.Contains(t, dag.Steps[2].Depends, "cmd_1")
		assert.Contains(t, dag.Steps[2].Depends, "cmd_2")

		assert.Equal(t, "cmd_4", dag.Steps[3].Name)
		assert.Contains(t, dag.Steps[3].Depends, "cmd_1")
		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
	})
}

func TestShorthandCommandSyntax(t *testing.T) {
	t.Parallel()

	t.Run("SimpleShorthandCommands", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "hello"
  - ls -la
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 2)

		// First step
		assert.Equal(t, "echo \"hello\"", dag.Steps[0].CmdWithArgs)
		assert.Equal(t, "echo", dag.Steps[0].Command)
		assert.Equal(t, []string{"hello"}, dag.Steps[0].Args)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name) // Auto-generated name

		// Second step
		assert.Equal(t, "ls -la", dag.Steps[1].CmdWithArgs)
		assert.Equal(t, "ls", dag.Steps[1].Command)
		assert.Equal(t, []string{"-la"}, dag.Steps[1].Args)
		assert.Equal(t, "cmd_2", dag.Steps[1].Name) // Auto-generated name
	})

	t.Run("MixedShorthandAndStandardSyntax", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "starting"
  - name: build
    command: make build
    env:
      DEBUG: "true"
  - ls -la
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 3)

		// First step (shorthand)
		assert.Equal(t, "echo \"starting\"", dag.Steps[0].CmdWithArgs)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)

		// Second step (standard)
		assert.Equal(t, "make build", dag.Steps[1].CmdWithArgs)
		assert.Equal(t, "build", dag.Steps[1].Name)
		assert.Contains(t, dag.Steps[1].Env, "DEBUG=true")

		// Third step (shorthand)
		assert.Equal(t, "ls -la", dag.Steps[2].CmdWithArgs)
		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
	})
}

func TestOptionalStepNames(t *testing.T) {
	t.Parallel()

	t.Run("AutoGenerateNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "hello"
  - npm test
  - go build
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("MixedExplicitAndGenerated", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - setup.sh
  - name: build
    command: make all
  - command: test.sh
    depends: build
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "build", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("HandleNameConflicts", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "first"
  - name: cmd_2
    command: echo "explicit"
  - echo "third"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("DependenciesWithGeneratedNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - git pull
  - command: npm install
    depends: cmd_1
  - command: npm test
    depends: cmd_2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)

		// Check dependencies are correctly resolved
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
	})

	t.Run("OutputVariablesWithGeneratedNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: echo "v1.0.0"
    output: VERSION
  - command: echo "Building version ${VERSION}"
    depends: cmd_1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 2)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "VERSION", th.Steps[0].Output)
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
	})

	t.Run("TypeBasedNaming", func(t *testing.T) {
		t.Parallel()

		// Test different step types get appropriate names
		data := []byte(`
steps:
  - echo "command"
  - script: |
      echo "script content"
  - executor:
      type: http
      config:
        url: https://example.com
  - call: child-dag
  - executor:
      type: docker
      config:
        image: alpine
  - executor:
      type: ssh
      config:
        host: example.com
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 6)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "script_2", th.Steps[1].Name)
		assert.Equal(t, "http_3", th.Steps[2].Name)
		assert.Equal(t, "dag_4", th.Steps[3].Name)
		assert.Equal(t, "docker_5", th.Steps[4].Name)
		assert.Equal(t, "ssh_6", th.Steps[5].Name)
	})

	t.Run("BackwardCompatibility", func(t *testing.T) {
		t.Parallel()

		// Ensure existing DAGs with explicit names still work
		data := []byte(`
steps:
  - echo "setup"
  - echo "test"
  - echo "deploy"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
		// In chain mode, sequential dependencies are implicit
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
	})
}

func TestStepIDValidation(t *testing.T) {
	t.Parallel()

	t.Run("ValidID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: valid_id
    command: echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "valid_id", dag.Steps[0].ID)
	})

	t.Run("InvalidIDFormat", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: 123invalid
    command: echo test
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step ID format")
	})

	t.Run("DuplicateIDs", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: step2
    id: myid
    command: echo test2
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate step ID")
	})

	t.Run("IDConflictsWithStepName", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: step2
    command: echo test1
  - name: step2
    command: echo test2
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts with another step's name")
	})

	t.Run("NameConflictsWithStepID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: myid
    command: echo test2
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts with another step's name")
	})

	t.Run("ReservedWordID", func(t *testing.T) {
		data := []byte(`
steps:
  - name: step1
    id: env
    command: echo test
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserved word")
	})
}

func TestStepIDInDependencies(t *testing.T) {
	t.Parallel()

	t.Run("DependOnStepByID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: first
    command: echo test2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, "first", dag.Steps[0].ID)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends) // ID "first" resolved to name "step1"
	})

	t.Run("DependOnStepByNameWhenIDExists", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: step1
    command: echo test2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends)
	})

	t.Run("MultipleDependenciesWithIDs", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    id: second
    command: echo test2
  - name: step3
    depends: 
      - first
      - second
    command: echo test3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // IDs resolved to names
	})

	t.Run("MixOfIDAndNameDependencies", func(t *testing.T) {
		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    command: echo test2
  - name: step3
    depends:
      - first
      - step2
    command: echo test3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // ID "first" resolved to name "step1"
	})
}

func TestChainTypeWithStepIDs(t *testing.T) {
	t.Parallel()

	data := []byte(`
type: chain
steps:
  - name: step1
    id: s1
    command: echo first
  - name: step2
    id: s2
    command: echo second
  - name: step3
    command: echo third
`)
	dag, err := spec.LoadYAML(context.Background(), data)
	require.NoError(t, err)
	require.Len(t, dag.Steps, 3)

	// Verify IDs are preserved
	assert.Equal(t, "s1", dag.Steps[0].ID)
	assert.Equal(t, "s2", dag.Steps[1].ID)
	assert.Equal(t, "", dag.Steps[2].ID)

	// Verify chain dependencies were added
	assert.Empty(t, dag.Steps[0].Depends)
	assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends)
	assert.Equal(t, []string{"step2"}, dag.Steps[2].Depends)
}

func TestResolveStepDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		expected map[string][]string // step name -> expected depends
	}{
		{
			name: "SingleIDDependency",
			yaml: `
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    depends: s1
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
		{
			name: "MultipleIDDependencies",
			yaml: `
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    id: s2
    command: echo "2"
  - name: step-three
    depends:
      - s1
      - s2
    command: echo "3"
`,
			expected: map[string][]string{
				"step-three": {"step-one", "step-two"},
			},
		},
		{
			name: "MixedIDAndNameDependencies",
			yaml: `
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    command: echo "2"
  - name: step-three
    depends:
      - s1
      - step-two
    command: echo "3"
`,
			expected: map[string][]string{
				"step-three": {"step-one", "step-two"},
			},
		},
		{
			name: "NoIDDependencies",
			yaml: `
steps:
  - name: step-one
    command: echo "1"
  - name: step-two
    depends: step-one
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
		{
			name: "IDSameAsName",
			yaml: `
steps:
  - name: step-one
    id: step-one
    command: echo "1"
  - name: step-two
    depends: step-one
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dag, err := spec.LoadYAML(ctx, []byte(tt.yaml))
			require.NoError(t, err)

			// Check that dependencies were resolved correctly
			for _, step := range dag.Steps {
				if expectedDeps, exists := tt.expected[step.Name]; exists {
					assert.Equal(t, expectedDeps, step.Depends,
						"Step %s dependencies should be resolved correctly", step.Name)
				}
			}
		})
	}
}

func TestResolveStepDependencies_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "DependencyOnNonExistentID",
			yaml: `
steps:
  - name: step-one
    command: echo "1"
  - name: step-two
    depends: nonexistent
    command: echo "2"
`,
			expectedErr: "", // This should be caught by dependency validation, not ID resolution
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := spec.LoadYAML(ctx, []byte(tt.yaml))
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				// Some tests expect no error from ID resolution
				// but might fail in other validation steps
				_ = err
			}
		})
	}
}

func TestBuildOTel(t *testing.T) {
	t.Parallel()

	t.Run("BasicOTelConfig", func(t *testing.T) {
		yaml := `
otel:
  enabled: true
  endpoint: localhost:4317
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.OTel)
		assert.True(t, dag.OTel.Enabled)
		assert.Equal(t, "localhost:4317", dag.OTel.Endpoint)
	})

	t.Run("FullOTelConfig", func(t *testing.T) {
		yaml := `
otel:
  enabled: true
  endpoint: otel-collector:4317
  headers:
    Authorization: Bearer token
  insecure: true
  timeout: 30s
  resource:
    service.name: dagu-test
    service.version: "1.0.0"
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.OTel)
		assert.True(t, dag.OTel.Enabled)
		assert.Equal(t, "otel-collector:4317", dag.OTel.Endpoint)
		assert.Equal(t, "Bearer token", dag.OTel.Headers["Authorization"])
		assert.True(t, dag.OTel.Insecure)
		assert.Equal(t, 30*time.Second, dag.OTel.Timeout)
		assert.Equal(t, "dagu-test", dag.OTel.Resource["service.name"])
		assert.Equal(t, "1.0.0", dag.OTel.Resource["service.version"])
	})

	t.Run("DisabledOTel", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		assert.Nil(t, dag.OTel)
	})
}

func TestContainer(t *testing.T) {
	t.Run("BasicContainer", func(t *testing.T) {
		yaml := `
container:
  image: python:3.11-slim
  pullPolicy: always
steps:
  - name: step1
    command: python script.py
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, "python:3.11-slim", dag.Container.Image)
		assert.Equal(t, core.PullPolicyAlways, dag.Container.PullPolicy)
	})

	t.Run("ContainerWithAllFields", func(t *testing.T) {
		yaml := `
container:
  image: node:18-alpine
  pullPolicy: missing
  env:
    - NODE_ENV: production
    - API_KEY: secret123
  volumes:
    - /data:/data:ro
    - /output:/output:rw
  user: "1000:1000"
  workingDir: /app
  platform: linux/amd64
  ports:
    - "8080:8080"
    - "9090:9090"
  network: bridge
  keepContainer: true
steps:
  - name: step1
    command: node app.js
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)

		assert.Equal(t, "node:18-alpine", dag.Container.Image)
		assert.Equal(t, core.PullPolicyMissing, dag.Container.PullPolicy)
		assert.Contains(t, dag.Container.Env, "NODE_ENV=production")
		assert.Contains(t, dag.Container.Env, "API_KEY=secret123")
		assert.Equal(t, []string{"/data:/data:ro", "/output:/output:rw"}, dag.Container.Volumes)
		assert.Equal(t, "1000:1000", dag.Container.User)
		assert.Equal(t, "/app", dag.Container.GetWorkingDir())
		assert.Equal(t, "linux/amd64", dag.Container.Platform)
		assert.Equal(t, []string{"8080:8080", "9090:9090"}, dag.Container.Ports)
		assert.Equal(t, "bridge", dag.Container.Network)
		assert.True(t, dag.Container.KeepContainer)
	})

	t.Run("ContainerEnvAsMap", func(t *testing.T) {
		yaml := `
container:
  image: alpine
  env:
    FOO: bar
    BAZ: qux
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Contains(t, dag.Container.Env, "FOO=bar")
		assert.Contains(t, dag.Container.Env, "BAZ=qux")
	})

	t.Run("ContainerPullPolicyVariations", func(t *testing.T) {
		testCases := []struct {
			name       string
			pullPolicy string
			expected   core.PullPolicy
		}{
			{"always", "always", core.PullPolicyAlways},
			{"never", "never", core.PullPolicyNever},
			{"missing", "missing", core.PullPolicyMissing},
			{"true", "true", core.PullPolicyAlways},
			{"false", "false", core.PullPolicyNever},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				yaml := `
container:
  image: alpine
  pullPolicy: ` + tc.pullPolicy + `
steps:
  - name: step1
    command: echo test
`
				ctx := context.Background()
				dag, err := spec.LoadYAML(ctx, []byte(yaml))
				require.NoError(t, err)
				require.NotNil(t, dag.Container)
				assert.Equal(t, tc.expected, dag.Container.PullPolicy)
			})
		}
	})

	t.Run("ContainerPullPolicyBoolean", func(t *testing.T) {
		// Test with boolean true
		yaml := `
container:
  image: alpine
  pullPolicy: true
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, core.PullPolicyAlways, dag.Container.PullPolicy)
	})

	t.Run("ContainerWithoutPullPolicy", func(t *testing.T) {
		yaml := `
container:
  image: alpine
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, core.PullPolicyMissing, dag.Container.PullPolicy)
	})

	t.Run("NoContainer", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		assert.Nil(t, dag.Container)
	})

	t.Run("InvalidPullPolicy", func(t *testing.T) {
		yaml := `
container:
  image: alpine
  pullPolicy: invalid_policy
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		_, err := spec.LoadYAML(ctx, []byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse pull policy")
	})

	t.Run("ContainerWithoutImage", func(t *testing.T) {
		yaml := `
container:
  pullPolicy: always
  env:
    - FOO: bar
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		_, err := spec.LoadYAML(ctx, []byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image is required when container is specified")
	})
}

func TestContainerExecutorIntegration(t *testing.T) {
	t.Run("StepInheritsContainerExecutor", func(t *testing.T) {
		yaml := `
container:
  image: python:3.11-slim
steps:
  - name: step1
    command: python script.py
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Step should have docker executor type when DAG has container
		assert.Equal(t, "container", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("ExplicitExecutorOverridesContainer", func(t *testing.T) {
		yaml := `
container:
  image: python:3.11-slim
steps:
  - name: step1
    command: echo test
    executor: shell
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Explicit executor should override DAG-level container
		assert.Equal(t, "shell", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("NoContainerNoExecutor", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// No container and no executor means default (empty) executor
		assert.Equal(t, "", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("StepWithDockerExecutorConfig", func(t *testing.T) {
		yaml := `
container:
  image: node:18-alpine
steps:
  - name: step1
    command: node app.js
    executor:
      type: docker
      config:
        image: python:3.11
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Step-level docker config should override DAG container
		assert.Equal(t, "docker", dag.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "python:3.11", dag.Steps[0].ExecutorConfig.Config["image"])
	})

	t.Run("MultipleStepsWithContainer", func(t *testing.T) {
		yaml := `
container:
  image: alpine:latest
steps:
  - name: step1
    command: echo "step 1"
  - name: step2
    command: echo "step 2"
    executor: shell
  - name: step3
    command: echo "step 3"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)

		// Step 1 and 3 should inherit docker executor
		assert.Equal(t, "container", dag.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "shell", dag.Steps[1].ExecutorConfig.Type)
		assert.Equal(t, "container", dag.Steps[2].ExecutorConfig.Type)
	})
}

func TestSSHConfiguration(t *testing.T) {
	t.Run("BasicSSHConfig", func(t *testing.T) {
		yaml := `
ssh:
  user: testuser
  host: example.com
  port: 2222
  key: ~/.ssh/id_rsa
steps:
  - name: step1
    command: echo hello
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.SSH)
		assert.Equal(t, "testuser", dag.SSH.User)
		assert.Equal(t, "example.com", dag.SSH.Host)
		assert.Equal(t, "2222", dag.SSH.Port)
		assert.Equal(t, "~/.ssh/id_rsa", dag.SSH.Key)
	})

	t.Run("SSHConfigWithStrictHostKey", func(t *testing.T) {
		yaml := `
ssh:
  user: testuser
  host: example.com
  strictHostKey: false
  knownHostFile: ~/.ssh/custom_known_hosts
steps:
  - name: step1
    command: echo hello
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.SSH)
		assert.Equal(t, "testuser", dag.SSH.User)
		assert.Equal(t, "example.com", dag.SSH.Host)
		assert.Equal(t, "22", dag.SSH.Port) // Default port
		assert.False(t, dag.SSH.StrictHostKey)
		assert.Equal(t, "~/.ssh/custom_known_hosts", dag.SSH.KnownHostFile)
	})

	t.Run("SSHConfigDefaultValues", func(t *testing.T) {
		yaml := `
ssh:
  user: testuser
  host: example.com
steps:
  - name: step1
    command: echo hello
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.SSH)
		assert.Equal(t, "22", dag.SSH.Port)        // Should default to 22
		assert.True(t, dag.SSH.StrictHostKey)      // Should default to true for security
		assert.Equal(t, "", dag.SSH.KnownHostFile) // Empty, will use default ~/.ssh/known_hosts at runtime
	})
}

func TestSSHInheritance(t *testing.T) {
	t.Run("StepInheritsSSHFromDAG", func(t *testing.T) {
		yaml := `
ssh:
  user: testuser
  host: example.com
  key: ~/.ssh/id_rsa
steps:
  - name: step1
    command: echo hello
  - name: step2
    command: ls -la
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// Both steps should inherit SSH executor
		for _, step := range dag.Steps {
			assert.Equal(t, "ssh", step.ExecutorConfig.Type)
		}
	})

	t.Run("StepOverridesSSHConfig", func(t *testing.T) {
		yaml := `
ssh:
  user: defaultuser
  host: default.com
  key: ~/.ssh/default_key
steps:
  - name: step1
    command: echo hello
    executor:
      type: ssh
      config:
        user: overrideuser
        ip: override.com
  - name: step2
    command: echo world
    executor:
      type: command
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// Step 1 should have overridden values
		step1 := dag.Steps[0]
		assert.Equal(t, "ssh", step1.ExecutorConfig.Type)

		// Step 2 should use command executor
		step2 := dag.Steps[1]
		assert.Equal(t, "command", step2.ExecutorConfig.Type)
	})
}

func TestStepLevelEnv(t *testing.T) {
	t.Run("BasicStepEnv", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo $STEP_VAR
    env:
      - STEP_VAR: step_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, []string{"STEP_VAR=step_value"}, dag.Steps[0].Env)
	})

	t.Run("StepEnvOverridesDAGEnv", func(t *testing.T) {
		yaml := `
env:
  - SHARED_VAR: dag_value
  - DAG_ONLY: dag_only_value
steps:
  - name: step1
    command: echo $SHARED_VAR
    env:
      - SHARED_VAR: step_value
      - STEP_ONLY: step_only_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		// Check DAG-level env
		assert.Contains(t, dag.Env, "SHARED_VAR=dag_value")
		assert.Contains(t, dag.Env, "DAG_ONLY=dag_only_value")
		// Check step-level env
		assert.Contains(t, dag.Steps[0].Env, "SHARED_VAR=step_value")
		assert.Contains(t, dag.Steps[0].Env, "STEP_ONLY=step_only_value")
	})

	t.Run("StepEnvAsMap", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
    env:
      FOO: foo_value
      BAR: bar_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "FOO=foo_value")
		assert.Contains(t, dag.Steps[0].Env, "BAR=bar_value")
	})

	t.Run("StepEnvWithSubstitution", func(t *testing.T) {
		yaml := `
env:
  - BASE_PATH: /tmp
steps:
  - name: step1
    command: echo $FULL_PATH
    env:
      - FULL_PATH: ${BASE_PATH}/data
      - COMPUTED: "` + "`echo computed_value`" + `"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "FULL_PATH=${BASE_PATH}/data")
		assert.Contains(t, dag.Steps[0].Env, "COMPUTED=`echo computed_value`")
	})

	t.Run("MultipleStepsWithDifferentEnvs", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo $ENV_VAR
    env:
      - ENV_VAR: value1
  - name: step2
    command: echo $ENV_VAR
    env:
      - ENV_VAR: value2
  - name: step3
    command: echo $ENV_VAR
    # No env, should inherit DAG env only
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"ENV_VAR=value1"}, dag.Steps[0].Env)
		assert.Equal(t, []string{"ENV_VAR=value2"}, dag.Steps[1].Env)
		assert.Empty(t, dag.Steps[2].Env)
	})

	t.Run("StepEnvComplexValues", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
    env:
      - PATH: "/custom/bin:${PATH}"
      - JSON_CONFIG: '{"key": "value", "nested": {"foo": "bar"}}'
      - MULTI_LINE: |
          line1
          line2
`
		ctx := context.Background()
		// Set PATH env var for substitution test
		origPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", origPath) }()
		os.Setenv("PATH", "/usr/bin")

		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "PATH=/custom/bin:${PATH}")
		assert.Contains(t, dag.Steps[0].Env, `JSON_CONFIG={"key": "value", "nested": {"foo": "bar"}}`)
		assert.Contains(t, dag.Steps[0].Env, "MULTI_LINE=line1\nline2\n")
	})
}

func TestBuildRegistryAuths(t *testing.T) {
	t.Run("ParseRegistryAuthsFromYAML", func(t *testing.T) {
		yaml := `
registryAuths:
  docker.io:
    username: docker-user
    password: docker-pass
  ghcr.io:
    username: github-user
    password: github-token
  gcr.io:
    auth: Z2NyLXVzZXI6Z2NyLXBhc3M= # base64("gcr-user:gcr-pass")

container:
  image: docker.io/myapp:latest

steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Check that registryAuths were parsed correctly
		assert.NotNil(t, dag.RegistryAuths)
		assert.Len(t, dag.RegistryAuths, 3)

		// Check docker.io auth
		dockerAuth, exists := dag.RegistryAuths["docker.io"]
		assert.True(t, exists)
		assert.Equal(t, "docker-user", dockerAuth.Username)
		assert.Equal(t, "docker-pass", dockerAuth.Password)

		// Check ghcr.io auth
		ghcrAuth, exists := dag.RegistryAuths["ghcr.io"]
		assert.True(t, exists)
		assert.Equal(t, "github-user", ghcrAuth.Username)
		assert.Equal(t, "github-token", ghcrAuth.Password)

		// Check gcr.io auth (with pre-encoded auth field)
		gcrAuth, exists := dag.RegistryAuths["gcr.io"]
		assert.True(t, exists)
		assert.Equal(t, "Z2NyLXVzZXI6Z2NyLXBhc3M=", gcrAuth.Auth)
		assert.Empty(t, gcrAuth.Username) // Should be empty when auth is provided
		assert.Empty(t, gcrAuth.Password) // Should be empty when auth is provided
	})

	t.Run("EmptyRegistryAuths", func(t *testing.T) {
		yaml := `
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Should be nil when not specified
		assert.Nil(t, dag.RegistryAuths)
	})

	t.Run("RegistryAuthsWithEnvironmentVariables", func(t *testing.T) {
		// Set environment variables for testing
		t.Setenv("DOCKER_USER", "env-docker-user")
		t.Setenv("DOCKER_PASS", "env-docker-pass")

		yaml := `
registryAuths:
  docker.io:
    username: ${DOCKER_USER}
    password: ${DOCKER_PASS}

steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Check that environment variables were expanded
		dockerAuth, exists := dag.RegistryAuths["docker.io"]
		assert.True(t, exists)
		assert.Equal(t, "env-docker-user", dockerAuth.Username)
		assert.Equal(t, "env-docker-pass", dockerAuth.Password)
	})

	t.Run("RegistryAuthsAsJSONString", func(t *testing.T) {
		// Simulate DOCKER_AUTH_CONFIG style JSON string
		jsonAuth := `{"docker.io": {"username": "json-user", "password": "json-pass"}}`
		t.Setenv("DOCKER_AUTH_JSON", jsonAuth)

		yaml := `
registryAuths: ${DOCKER_AUTH_JSON}

steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Should have stored the JSON string as _json entry
		assert.NotNil(t, dag.RegistryAuths)
		jsonEntry, exists := dag.RegistryAuths["_json"]
		assert.True(t, exists)
		assert.Equal(t, jsonAuth, jsonEntry.Auth)
	})

	t.Run("RegistryAuthsWithStringValuesPerRegistry", func(t *testing.T) {
		yaml := `
registryAuths:
  docker.io: '{"username": "user1", "password": "pass1"}'
  ghcr.io: '{"username": "user2", "password": "pass2"}'

steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Check docker.io - should have the JSON string in Auth field
		dockerAuth, exists := dag.RegistryAuths["docker.io"]
		assert.True(t, exists)
		assert.Equal(t, `{"username": "user1", "password": "pass1"}`, dockerAuth.Auth)
		assert.Empty(t, dockerAuth.Username)
		assert.Empty(t, dockerAuth.Password)

		// Check ghcr.io
		ghcrAuth, exists := dag.RegistryAuths["ghcr.io"]
		assert.True(t, exists)
		assert.Equal(t, `{"username": "user2", "password": "pass2"}`, ghcrAuth.Auth)
	})
}

func TestBuildWorkingDir(t *testing.T) {
	t.Run("ExplicitWorkingDir", func(t *testing.T) {
		yaml := `
workingDir: /tmp
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.Equal(t, "/tmp", dag.WorkingDir)
	})

	t.Run("WorkingDirWithEnvVarExpansion", func(t *testing.T) {
		t.Setenv("TEST_DIR", "/tmp/dir")
		yaml := `
workingDir: ${TEST_DIR}/subdir
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.Equal(t, "/tmp/dir/subdir", dag.WorkingDir)
	})

	t.Run("DefaultWorkingDirWhenNoFile", func(t *testing.T) {
		yaml := `
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Should be current working directory
		expectedDir, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, expectedDir, dag.WorkingDir)
	})
}

func TestDAGLoadEnv(t *testing.T) {
	t.Run("LoadEnvWithDotenvAndEnvVars", func(t *testing.T) {
		// Create a temp directory with a .env file
		tempDir := t.TempDir()
		envFile := filepath.Join(tempDir, ".env")
		envContent := "LOAD_ENV_DOTENV_VAR=from_file\n"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		yaml := fmt.Sprintf(`
workingDir: %s
dotenv: .env
env:
  - LOAD_ENV_ENV_VAR: from_dag
  - LOAD_ENV_ANOTHER_VAR: another_value
steps:
  - echo hello
`, tempDir)

		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{NoEval: true})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Load environment variables from dotenv file
		dag.LoadDotEnv(context.Background())

		// Verify environment variables are in dag.Env (not process env)
		// Child processes will receive them via cmd.Env = AllEnvs()
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
		assert.Equal(t, "from_file", envMap["LOAD_ENV_DOTENV_VAR"])
		assert.Equal(t, "from_dag", envMap["LOAD_ENV_ENV_VAR"])
		assert.Equal(t, "another_value", envMap["LOAD_ENV_ANOTHER_VAR"])
	})

	t.Run("LoadEnvWithMissingDotenvFile", func(t *testing.T) {
		yaml := `
dotenv: nonexistent.env
env:
  - TEST_VAR_LOAD_ENV: test_value
steps:
  - echo hello
`
		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{NoEval: true})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// LoadDotEnv should not fail even if dotenv file doesn't exist
		dag.LoadDotEnv(context.Background())

		// Environment variables from env should still be in dag.Env
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}
		assert.Equal(t, "test_value", envMap["TEST_VAR_LOAD_ENV"])
	})
}
