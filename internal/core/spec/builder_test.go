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
	t.Parallel()

	// Simple field tests grouped into table-driven format
	simpleTests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, dag *core.DAG)
	}{
		{
			name: "SkipIfSuccessful",
			yaml: `
skipIfSuccessful: true
steps:
  - "true"
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.True(t, dag.SkipIfSuccessful)
			},
		},
		{
			name: "MailOnBoth",
			yaml: `
steps:
  - "true"
mailOn:
  failure: true
  success: true
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.True(t, dag.MailOn.Failure)
				assert.True(t, dag.MailOn.Success)
			},
		},
		{
			name: "ValidTags",
			yaml: `
tags: daily,monthly
steps:
  - echo 1
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.True(t, dag.HasTag("daily"))
				assert.True(t, dag.HasTag("monthly"))
			},
		},
		{
			name: "ValidTagsList",
			yaml: `
tags:
  - daily
  - monthly
steps:
  - echo 1
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.True(t, dag.HasTag("daily"))
				assert.True(t, dag.HasTag("monthly"))
			},
		},
		{
			name: "LogDir",
			yaml: `
logDir: /tmp/logs
steps:
  - "true"
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, "/tmp/logs", dag.LogDir)
			},
		},
		{
			name: "MaxHistRetentionDays",
			yaml: `
histRetentionDays: 365
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 365, dag.HistRetentionDays)
			},
		},
		{
			name: "CleanUpTime",
			yaml: `
maxCleanUpTimeSec: 10
steps:
  - "true"
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 10*time.Second, dag.MaxCleanUpTime)
			},
		},
		{
			name: "DefaultTypeIsChain",
			yaml: `
steps:
  - echo 1
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, core.TypeChain, dag.Type)
			},
		},
		{
			name: "MaxActiveRuns",
			yaml: `
maxActiveRuns: 5
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 5, dag.MaxActiveRuns)
			},
		},
		{
			name: "MaxActiveSteps",
			yaml: `
maxActiveSteps: 3
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 3, dag.MaxActiveSteps)
			},
		},
		{
			name: "RunConfig",
			yaml: `
runConfig:
  disableParamEdit: true
  disableRunIdEdit: true
`,
			check: func(t *testing.T, dag *core.DAG) {
				require.NotNil(t, dag.RunConfig)
				assert.True(t, dag.RunConfig.DisableParamEdit)
				assert.True(t, dag.RunConfig.DisableRunIdEdit)
			},
		},
		{
			name: "MaxOutputSizeCustom",
			yaml: `
description: Test DAG with custom maxOutputSize
maxOutputSize: 524288
steps:
  - echo "test output"
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 524288, dag.MaxOutputSize)
			},
		},
		{
			name: "MaxOutputSizeDefault",
			yaml: `
steps:
  - "true"
`,
			check: func(t *testing.T, dag *core.DAG) {
				assert.Equal(t, 0, dag.MaxOutputSize)
			},
		},
		{
			name: "Preconditions",
			yaml: `
preconditions:
  - condition: "test -f file.txt"
    expected: "true"
`,
			check: func(t *testing.T, dag *core.DAG) {
				require.Len(t, dag.Preconditions, 1)
				assert.Equal(t, &core.Condition{Condition: "test -f file.txt", Expected: "true"}, dag.Preconditions[0])
			},
		},
		{
			name: "PreconditionsWithNegate",
			yaml: `
preconditions:
  - condition: "${STATUS}"
    expected: "success"
    negate: true
`,
			check: func(t *testing.T, dag *core.DAG) {
				require.Len(t, dag.Preconditions, 1)
				assert.Equal(t, &core.Condition{Condition: "${STATUS}", Expected: "success", Negate: true}, dag.Preconditions[0])
			},
		},
	}

	for _, tt := range simpleTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			tt.check(t, dag)
		})
	}

	// Params tests grouped into table-driven format
	paramTests := []struct {
		name       string
		yaml       string
		opts       *spec.BuildOpts
		wantParams []string
	}{
		{
			name: "ParamsWithSubstitution",
			yaml: `
params: "TEST_PARAM $1"
`,
			wantParams: []string{"1=TEST_PARAM", "2=TEST_PARAM"},
		},
		{
			name: "ParamsWithQuotedValues",
			yaml: `
params: x="a b c" y="d e f"
`,
			wantParams: []string{"x=a b c", "y=d e f"},
		},
		{
			name: "ParamsAsMap",
			yaml: `
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`,
			wantParams: []string{"FOO=foo", "BAR=bar", "BAZ=baz"},
		},
		{
			name: "ParamsAsMapOverride",
			yaml: `
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`,
			opts:       &spec.BuildOpts{Parameters: "FOO=X BAZ=Y"},
			wantParams: []string{"FOO=X", "BAR=bar", "BAZ=Y"},
		},
		{
			name: "ParamsWithComplexValues",
			yaml: `
params: first P1=foo P2=${A001} P3=` + "`/bin/echo BAR`" + ` X=bar Y=${P1} Z="A B C"
env:
  - A001: TEXT
`,
			wantParams: []string{"1=first", "P1=foo", "P2=TEXT", "P3=BAR", "X=bar", "Y=foo", "Z=A B C"},
		},
		{
			name: "ParamsWithSubstringAndDefaults",
			yaml: `
env:
  - SOURCE_ID: HBL01_22OCT2025_0536
params:
  - BASE: ${SOURCE_ID}
  - PREFIX: ${BASE:0:5}
  - REMAINDER: ${BASE:5}
  - FALLBACK: ${MISSING_VALUE:-fallback}
`,
			wantParams: []string{"BASE=HBL01_22OCT2025_0536", "PREFIX=HBL01", "REMAINDER=_22OCT2025_0536", "FALLBACK=fallback"},
		},
		{
			name: "ParamsNoEvalPreservesRaw",
			yaml: `
env:
  - SOURCE_ID: HBL01_22OCT2025_0536
params:
  - BASE: ${SOURCE_ID}
  - PREFIX: ${BASE:0:5}
`,
			opts:       &spec.BuildOpts{Flags: spec.BuildFlagNoEval},
			wantParams: []string{"BASE=${SOURCE_ID}", "PREFIX=${BASE:0:5}"},
		},
	}

	for _, tt := range paramTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var dag *core.DAG
			var err error
			if tt.opts != nil {
				dag, err = spec.LoadYAMLWithOpts(context.Background(), []byte(tt.yaml), *tt.opts)
			} else {
				dag, err = spec.LoadYAML(context.Background(), []byte(tt.yaml))
			}
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
			th.AssertParam(t, tt.wantParams...)
		})
	}
}

func TestBuildParamsWithLocalSchemaReference(t *testing.T) {
	t.Parallel()

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

	require.Len(t, th.Params, 2)
	require.Contains(t, th.Params, "batch_size=25")
	require.Contains(t, th.Params, "environment=staging")
}

func TestBuildParamsWithRemoteSchemaReference(t *testing.T) {
	t.Parallel()

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

	require.Len(t, th.Params, 2)
	require.Contains(t, th.Params, "batch_size=50")
	require.Contains(t, th.Params, "environment=prod")
}

func TestBuildParamsSchemaResolution(t *testing.T) {
	t.Run("FromWorkingDir", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {"type": "integer", "default": 42}
  }
}`

		wd := t.TempDir()
		wdSchema := filepath.Join(wd, "schema.json")
		require.NoError(t, os.WriteFile(wdSchema, []byte(schemaContent), 0600))

		origWD, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() {
			if err := os.Chdir(origWD); err != nil {
				t.Fatalf("failed to restore working directory: %v", err)
			}
		})

		data := []byte(fmt.Sprintf(`
workingDir: %s
params:
  schema: "schema.json"
  values:
    environment: "dev"
`, wd))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Contains(t, th.Params, "batch_size=42")
	})

	t.Run("FromDAGDir", func(t *testing.T) {
		schemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {"type": "integer", "default": 7}
  }
}`

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(schemaContent), 0600))

		dagYaml := []byte(`
params:
  schema: "schema.json"
  values:
    environment: "staging"
`)
		dagPath := filepath.Join(dir, "dag.yaml")
		require.NoError(t, os.WriteFile(dagPath, dagYaml, 0600))

		dag, err := spec.Load(context.Background(), dagPath)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Contains(t, th.Params, "batch_size=7")
	})

	t.Run("PrefersCWDOverWorkingDir", func(t *testing.T) {
		cwdSchemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {"type": "integer", "default": 99}
  }
}`
		wdSchemaContent := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "batch_size": {"type": "integer", "default": 11}
  }
}`

		cwd := t.TempDir()
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(cwd, "schema.json"), []byte(cwdSchemaContent), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(wd, "schema.json"), []byte(wdSchemaContent), 0600))

		orig, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(cwd))
		defer os.Chdir(orig)

		data := []byte(fmt.Sprintf(`
workingDir: %s
params:
  schema: "schema.json"
  values:
    environment: "dev"
`, wd))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Contains(t, th.Params, "batch_size=99")
	})
}

func TestBuildParamsSchemaValidation(t *testing.T) {
	t.Parallel()

	t.Run("SkipSchemaValidationFlag", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
params:
  schema: "missing-schema.json"
  values:
    foo: "bar"
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)

		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{
			Flags: spec.BuildFlagSkipSchemaValidation,
		})
		require.NoError(t, err)

		th := DAG{t: t, DAG: dag}
		th.AssertParam(t, "foo=bar")
	})

	t.Run("OverrideValidationFails", func(t *testing.T) {
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

		cliParams := "batch_size=100 environment=prod"
		_, err = spec.LoadYAML(context.Background(), data, spec.WithParams(cliParams))
		require.Error(t, err)
		require.Contains(t, err.Error(), "parameter validation failed")
		require.Contains(t, err.Error(), "maximum: 100/1 is greater than 50")
	})

	t.Run("DefaultsApplied", func(t *testing.T) {
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

		tmpFile, err := os.CreateTemp("", "test-schema-defaults-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 75
`, tmpFile.Name()))

		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Params, 3)
		require.Contains(t, th.Params, "batch_size=75")
		require.Contains(t, th.Params, "environment=development")
		require.Contains(t, th.Params, "debug=true")
	})

	t.Run("DefaultsPreserveExistingValues", func(t *testing.T) {
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

		tmpFile, err := os.CreateTemp("", "test-schema-preserve-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		tmpFile.Close()

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

		require.Len(t, th.Params, 4)
		require.Contains(t, th.Params, "batch_size=50")
		require.Contains(t, th.Params, "environment=production")
		require.Contains(t, th.Params, "debug=false")
		require.Contains(t, th.Params, "timeout=600")
	})
}

func TestBuildMailConfig(t *testing.T) {
	t.Parallel()

	t.Run("BasicConfig", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
smtp:
  host: "smtp.example.com"
  port: "587"
  username: user@example.com
  password: password
errorMail:
  from: "error@example.com"
  to: "admin@example.com"
  prefix: "[ERROR]"
  attachLogs: true
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

	t.Run("NumericPort", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
smtp:
  host: "smtp.example.com"
  port: 587
  username: "user@example.com"
  password: "password"
steps:
  - echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.SMTP)
		assert.Equal(t, "smtp.example.com", dag.SMTP.Host)
		assert.Equal(t, "587", dag.SMTP.Port)
		assert.Equal(t, "user@example.com", dag.SMTP.Username)
		assert.Equal(t, "password", dag.SMTP.Password)
	})

	t.Run("MultipleRecipients", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
smtp:
  host: "smtp.example.com"
  port: "587"
  username: user@example.com
  password: password
errorMail:
  from: "error@example.com"
  to:
    - "admin1@example.com"
    - "admin2@example.com"
    - "admin3@example.com"
  prefix: "[ERROR]"
  attachLogs: true
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

		assert.Equal(t, "error@example.com", th.ErrorMail.From)
		assert.Equal(t, []string{"admin1@example.com", "admin2@example.com", "admin3@example.com"}, th.ErrorMail.To)
		assert.Equal(t, "[ERROR]", th.ErrorMail.Prefix)
		assert.True(t, th.ErrorMail.AttachLogs)

		assert.Equal(t, "info@example.com", th.InfoMail.From)
		assert.Equal(t, []string{"user@example.com"}, th.InfoMail.To)
		assert.Equal(t, "[INFO]", th.InfoMail.Prefix)
		assert.False(t, th.InfoMail.AttachLogs)
	})
}

func TestBuildChainType(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

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

		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
		assert.Equal(t, []string{"cmd_3"}, th.Steps[3].Depends)
	})

	t.Run("WithExplicitDepends", func(t *testing.T) {
		t.Parallel()

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
    depends:
      - download-a
      - download-b
  - name: cleanup
    command: rm -f fileA fileB
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		assert.Len(t, th.Steps, 5)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"setup"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"download-a"}, th.Steps[2].Depends)
		assert.ElementsMatch(t, []string{"download-a", "download-b"}, th.Steps[3].Depends)
		assert.Equal(t, []string{"process-both"}, th.Steps[4].Depends)
	})

	t.Run("InvalidType", func(t *testing.T) {
		t.Parallel()

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

	t.Run("WithEmptyDependencies", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
type: chain
steps:
  - name: step1
    command: echo "First"
  - name: step2
    command: echo "Second"
  - name: step3
    command: echo "Third"
    depends: []
  - name: step4
    command: echo "Fourth"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends)
		assert.Empty(t, th.Steps[2].Depends)
		assert.Equal(t, []string{"step3"}, th.Steps[3].Depends)
	})
}

func TestBuildValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		expectedErr error
	}{
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			if errs, ok := err.(*core.ErrorList); ok && len(*errs) > 0 {
				found := false
				for _, e := range *errs {
					if errors.Is(e, tt.expectedErr) {
						found = true
						break
					}
				}
				require.True(t, found, "expected error %v, got %v", tt.expectedErr, err)
			} else {
				assert.ErrorIs(t, err, tt.expectedErr)
			}
		})
	}
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
		assert.Equal(t, "call", th.Steps[0].Command)
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
		assert.Equal(t, "call", thLegacy.Steps[0].Command)
		assert.Equal(t, []string{"sub_dag_legacy", ""}, thLegacy.Steps[0].Args)
		assert.Equal(t, "sub_dag_legacy", thLegacy.Steps[0].CmdWithArgs)
		require.Len(t, dagLegacy.BuildWarnings, 1)
		assert.Contains(t, dagLegacy.BuildWarnings[0], "Step field 'run' is deprecated")
	})
	// ContinueOn success cases
	continueOnTests := []struct {
		name        string
		yaml        string
		wantSkipped bool
		wantFailure bool
		wantExitCode []int
		wantMarkSuccess bool
	}{
		{
			name: "ContinueOnObject",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      skipped: true
      failure: true
`,
			wantSkipped: true,
			wantFailure: true,
		},
		{
			name: "ContinueOnStringSkipped",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: skipped
`,
			wantSkipped: true,
			wantFailure: false,
		},
		{
			name: "ContinueOnStringFailed",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: failed
`,
			wantSkipped: false,
			wantFailure: true,
		},
		{
			name: "ContinueOnStringCaseInsensitive",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: SKIPPED
`,
			wantSkipped: true,
		},
		{
			name: "ContinueOnObjectWithExitCode",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      exitCode: [1, 2, 3]
      markSuccess: true
`,
			wantExitCode:    []int{1, 2, 3},
			wantMarkSuccess: true,
		},
	}

	for _, tt := range continueOnTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			assert.Equal(t, tt.wantSkipped, dag.Steps[0].ContinueOn.Skipped)
			assert.Equal(t, tt.wantFailure, dag.Steps[0].ContinueOn.Failure)
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, dag.Steps[0].ContinueOn.ExitCode)
			}
			if tt.wantMarkSuccess {
				assert.True(t, dag.Steps[0].ContinueOn.MarkSuccess)
			}
		})
	}

	// ContinueOn error cases
	continueOnErrorTests := []struct {
		name         string
		yaml         string
		errContains  []string
	}{
		{
			name: "ContinueOnInvalidString",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: invalid
`,
			errContains: []string{"continueOn"},
		},
		{
			name: "ContinueOnInvalidFailureType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      failure: "true"
`,
			errContains: []string{"continueOn.failure", "boolean"},
		},
		{
			name: "ContinueOnInvalidSkippedType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      skipped: 1
`,
			errContains: []string{"continueOn.skipped", "boolean"},
		},
		{
			name: "ContinueOnInvalidMarkSuccessType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      markSuccess: "yes"
`,
			errContains: []string{"continueOn.markSuccess", "boolean"},
		},
	}

	for _, tt := range continueOnErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			for _, s := range tt.errContains {
				assert.Contains(t, err.Error(), s)
			}
		})
	}
	// RetryPolicy success tests
	retryPolicyTests := []struct {
		name            string
		yaml            string
		wantLimit       int
		wantInterval    time.Duration
		wantBackoff     float64
		wantMaxInterval time.Duration
	}{
		{
			name: "RetryPolicyBasic",
			yaml: `
steps:
  - command: "echo 2"
    retryPolicy:
      limit: 3
      intervalSec: 10
`,
			wantLimit:    3,
			wantInterval: 10 * time.Second,
		},
		{
			name: "RetryPolicyWithBackoff",
			yaml: `
steps:
  - name: "test_backoff"
    command: "echo test"
    retryPolicy:
      limit: 5
      intervalSec: 2
      backoff: 2.0
      maxIntervalSec: 30
`,
			wantLimit:       5,
			wantInterval:    2 * time.Second,
			wantBackoff:     2.0,
			wantMaxInterval: 30 * time.Second,
		},
		{
			name: "RetryPolicyWithBackoffBool",
			yaml: `
steps:
  - name: "test_backoff_bool"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: true
      maxIntervalSec: 10
`,
			wantLimit:       3,
			wantInterval:    1 * time.Second,
			wantBackoff:     2.0, // true converts to 2.0
			wantMaxInterval: 10 * time.Second,
		},
	}

	for _, tt := range retryPolicyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			require.NotNil(t, dag.Steps[0].RetryPolicy)
			assert.Equal(t, tt.wantLimit, dag.Steps[0].RetryPolicy.Limit)
			assert.Equal(t, tt.wantInterval, dag.Steps[0].RetryPolicy.Interval)
			if tt.wantBackoff > 0 {
				assert.Equal(t, tt.wantBackoff, dag.Steps[0].RetryPolicy.Backoff)
			}
			if tt.wantMaxInterval > 0 {
				assert.Equal(t, tt.wantMaxInterval, dag.Steps[0].RetryPolicy.MaxInterval)
			}
		})
	}

	// RetryPolicy error tests
	retryPolicyErrorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "RetryPolicyInvalidBackoff",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: 0.8
`,
			errContains: "backoff must be greater than 1.0",
		},
		{
			name: "RetryPolicyMissingLimit",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      intervalSec: 5
`,
			errContains: "limit is required when retryPolicy is specified",
		},
		{
			name: "RetryPolicyMissingIntervalSec",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
`,
			errContains: "intervalSec is required when retryPolicy is specified",
		},
	}

	for _, tt := range retryPolicyErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Nil(t, dag)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
	// RepeatPolicy success tests
	repeatPolicyTests := []struct {
		name            string
		yaml            string
		wantMode        core.RepeatMode
		wantInterval    time.Duration
		wantLimit       int
		wantExitCode    []int
		wantCondition   string
		wantExpected    string
		wantBackoff     float64
		wantMaxInterval time.Duration
		wantNoCondition bool
	}{
		{
			name: "RepeatPolicyBasic",
			yaml: `
steps:
  - command: "echo 2"
    repeatPolicy:
      repeat: true
      intervalSec: 60
`,
			wantMode:     core.RepeatModeWhile,
			wantInterval: 60 * time.Second,
			wantLimit:    0,
		},
		{
			name: "RepeatPolicyWhileCondition",
			yaml: `
steps:
  - name: "repeat-while-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      condition: "echo hello"
      intervalSec: 5
      limit: 3
`,
			wantMode:      core.RepeatModeWhile,
			wantInterval:  5 * time.Second,
			wantLimit:     3,
			wantCondition: "echo hello",
			wantExpected:  "",
		},
		{
			name: "RepeatPolicyUntilCondition",
			yaml: `
steps:
  - name: "repeat-until-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      condition: "echo hello"
      expected: "hello"
      intervalSec: 10
      limit: 5
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  10 * time.Second,
			wantLimit:     5,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyWhileExitCode",
			yaml: `
steps:
  - name: "repeat-while-exitcode"
    command: "exit 1"
    repeatPolicy:
      repeat: "while"
      exitCode: [1, 2]
      intervalSec: 15
`,
			wantMode:        core.RepeatModeWhile,
			wantInterval:    15 * time.Second,
			wantExitCode:    []int{1, 2},
			wantNoCondition: true,
		},
		{
			name: "RepeatPolicyUntilExitCode",
			yaml: `
steps:
  - name: "repeat-until-exitcode"
    command: "exit 0"
    repeatPolicy:
      repeat: "until"
      exitCode: [0]
      intervalSec: 20
`,
			wantMode:        core.RepeatModeUntil,
			wantInterval:    20 * time.Second,
			wantExitCode:    []int{0},
			wantNoCondition: true,
		},
		{
			name: "RepeatPolicyBackwardCompatibilityUntil",
			yaml: `
steps:
  - name: "repeat-backward-compatibility-until"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 25
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  25 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyBackwardCompatibilityWhile",
			yaml: `
steps:
  - name: "repeat-backward-compatibility-while"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      intervalSec: 30
`,
			wantMode:      core.RepeatModeWhile,
			wantInterval:  30 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "",
		},
		{
			name: "RepeatPolicyCondition",
			yaml: `
steps:
  - name: "repeat-condition"
    command: "echo hello"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 1
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  1 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyExitCode",
			yaml: `
steps:
  - name: "repeat-exitcode"
    command: "exit 42"
    repeatPolicy:
      exitCode: [42]
      intervalSec: 2
`,
			wantMode:     core.RepeatModeWhile,
			wantInterval: 2 * time.Second,
			wantExitCode: []int{42},
		},
		{
			name: "RepeatPolicyWithBackoff",
			yaml: `
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
`,
			wantMode:        core.RepeatModeWhile,
			wantInterval:    5 * time.Second,
			wantBackoff:     1.5,
			wantMaxInterval: 60 * time.Second,
			wantLimit:       10,
			wantExitCode:    []int{1},
		},
		{
			name: "RepeatPolicyWithBackoffBool",
			yaml: `
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
`,
			wantMode:        core.RepeatModeUntil,
			wantInterval:    2 * time.Second,
			wantBackoff:     2.0,
			wantMaxInterval: 20 * time.Second,
			wantLimit:       5,
			wantCondition:   "echo done",
			wantExpected:    "done",
		},
	}

	for _, tt := range repeatPolicyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			rp := dag.Steps[0].RepeatPolicy
			require.NotNil(t, rp)
			assert.Equal(t, tt.wantMode, rp.RepeatMode)
			assert.Equal(t, tt.wantInterval, rp.Interval)
			assert.Equal(t, tt.wantLimit, rp.Limit)
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, rp.ExitCode)
			}
			if tt.wantNoCondition {
				assert.Nil(t, rp.Condition)
			} else if tt.wantCondition != "" {
				require.NotNil(t, rp.Condition)
				assert.Equal(t, tt.wantCondition, rp.Condition.Condition)
				assert.Equal(t, tt.wantExpected, rp.Condition.Expected)
			}
			if tt.wantBackoff > 0 {
				assert.Equal(t, tt.wantBackoff, rp.Backoff)
			}
			if tt.wantMaxInterval > 0 {
				assert.Equal(t, tt.wantMaxInterval, rp.MaxInterval)
			}
		})
	}
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
	t.Run("StepPreconditionsWithNegate", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "step_with_negate"
    command: "echo hello"
    preconditions:
      - condition: "${STATUS}"
        expected: "success"
        negate: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &core.Condition{Condition: "${STATUS}", Expected: "success", Negate: true}, th.Steps[0].Preconditions[0])
	})
	// RepeatPolicy error tests
	repeatPolicyErrorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "RepeatPolicyInvalidRepeatValue",
			yaml: `
steps:
  - name: "invalid-repeat"
    command: "echo test"
    repeatPolicy:
      repeat: "invalid"
      intervalSec: 10
`,
			errContains: "invalid value for repeat: 'invalid'",
		},
		{
			name: "RepeatPolicyWhileNoCondition",
			yaml: `
steps:
  - name: "while-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 10
`,
			errContains: "repeat mode 'while' requires either 'condition' or 'exitCode' to be specified",
		},
		{
			name: "RepeatPolicyUntilNoCondition",
			yaml: `
steps:
  - name: "until-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      intervalSec: 10
`,
			errContains: "repeat mode 'until' requires either 'condition' or 'exitCode' to be specified",
		},
		{
			name: "RepeatPolicyInvalidType",
			yaml: `
steps:
  - name: "invalid-type"
    command: "echo test"
    repeatPolicy:
      repeat: 123
      intervalSec: 10
`,
			errContains: "invalid value for repeat",
		},
		{
			name: "RepeatPolicyBackoffTooLow",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 1.0
      exitCode: [1]
`,
			errContains: "backoff must be greater than 1.0",
		},
		{
			name: "RepeatPolicyBackoffBelowOne",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 0.5
      exitCode: [1]
`,
			errContains: "backoff must be greater than 1.0",
		},
	}

	for _, tt := range repeatPolicyErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Nil(t, dag)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
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
  - call: sub-dag
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

	// Success test
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

	// Error tests
	errorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "InvalidIDFormat",
			yaml: `
steps:
  - name: step1
    id: 123invalid
    command: echo test
`,
			errContains: "invalid step ID format",
		},
		{
			name: "DuplicateIDs",
			yaml: `
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: step2
    id: myid
    command: echo test2
`,
			errContains: "duplicate step ID",
		},
		{
			name: "IDConflictsWithStepName",
			yaml: `
steps:
  - name: step1
    id: step2
    command: echo test1
  - name: step2
    command: echo test2
`,
			errContains: "conflicts with another step's name",
		},
		{
			name: "NameConflictsWithStepID",
			yaml: `
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: myid
    command: echo test2
`,
			errContains: "conflicts with another step's name",
		},
		{
			name: "ReservedWordID",
			yaml: `
steps:
  - name: step1
    id: env
    command: echo test
`,
			errContains: "reserved word",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
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

	tests := []struct {
		name         string
		yaml         string
		wantNil      bool
		wantEnabled  bool
		wantEndpoint string
		checkOTel    func(t *testing.T, otel *core.OTelConfig)
	}{
		{
			name: "BasicConfig",
			yaml: `
otel:
  enabled: true
  endpoint: localhost:4317
steps:
  - name: step1
    command: echo "test"
`,
			wantEnabled:  true,
			wantEndpoint: "localhost:4317",
		},
		{
			name: "FullConfig",
			yaml: `
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
`,
			wantEnabled:  true,
			wantEndpoint: "otel-collector:4317",
			checkOTel: func(t *testing.T, otel *core.OTelConfig) {
				assert.Equal(t, "Bearer token", otel.Headers["Authorization"])
				assert.True(t, otel.Insecure)
				assert.Equal(t, 30*time.Second, otel.Timeout)
				assert.Equal(t, "dagu-test", otel.Resource["service.name"])
				assert.Equal(t, "1.0.0", otel.Resource["service.version"])
			},
		},
		{
			name: "Disabled",
			yaml: `
steps:
  - name: step1
    command: echo "test"
`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, dag.OTel)
				return
			}
			require.NotNil(t, dag.OTel)
			assert.Equal(t, tt.wantEnabled, dag.OTel.Enabled)
			assert.Equal(t, tt.wantEndpoint, dag.OTel.Endpoint)
			if tt.checkOTel != nil {
				tt.checkOTel(t, dag.OTel)
			}
		})
	}
}

func TestContainer(t *testing.T) {
	t.Parallel()

	// Basic container tests
	basicTests := []struct {
		name           string
		yaml           string
		wantImage      string
		wantName       string
		wantPullPolicy core.PullPolicy
		wantNil        bool
		wantEnv        []string
	}{
		{
			name: "BasicContainer",
			yaml: `
container:
  image: python:3.11-slim
  pullPolicy: always
steps:
  - name: step1
    command: python script.py
`,
			wantImage:      "python:3.11-slim",
			wantPullPolicy: core.PullPolicyAlways,
		},
		{
			name: "ContainerWithName",
			yaml: `
container:
  name: my-dag-container
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "my-dag-container",
		},
		{
			name: "ContainerNameEmpty",
			yaml: `
container:
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "",
		},
		{
			name: "ContainerNameTrimmed",
			yaml: `
container:
  name: "  my-container  "
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "my-container",
		},
		{
			name: "ContainerEnvAsMap",
			yaml: `
container:
  image: alpine
  env:
    FOO: bar
    BAZ: qux
steps:
  - name: step1
    command: echo test
`,
			wantImage: "alpine",
			wantEnv:   []string{"FOO=bar", "BAZ=qux"},
		},
		{
			name: "ContainerWithoutPullPolicy",
			yaml: `
container:
  image: alpine
steps:
  - name: step1
    command: echo test
`,
			wantImage:      "alpine",
			wantPullPolicy: core.PullPolicyMissing,
		},
		{
			name: "NoContainer",
			yaml: `
steps:
  - name: step1
    command: echo test
`,
			wantNil: true,
		},
	}

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, dag.Container)
				return
			}
			require.NotNil(t, dag.Container)
			if tt.wantImage != "" {
				assert.Equal(t, tt.wantImage, dag.Container.Image)
			}
			if tt.wantName != "" || tt.name == "ContainerNameEmpty" {
				assert.Equal(t, tt.wantName, dag.Container.Name)
			}
			if tt.wantPullPolicy != 0 {
				assert.Equal(t, tt.wantPullPolicy, dag.Container.PullPolicy)
			}
			for _, env := range tt.wantEnv {
				assert.Contains(t, dag.Container.Env, env)
			}
		})
	}

	// Pull policy variations
	pullPolicyTests := []struct {
		name       string
		pullPolicy string
		expected   core.PullPolicy
	}{
		{"Always", "always", core.PullPolicyAlways},
		{"Never", "never", core.PullPolicyNever},
		{"Missing", "missing", core.PullPolicyMissing},
		{"TrueString", "true", core.PullPolicyAlways},
		{"FalseString", "false", core.PullPolicyNever},
	}

	for _, tt := range pullPolicyTests {
		t.Run("PullPolicy"+tt.name, func(t *testing.T) {
			t.Parallel()
			yaml := `
container:
  image: alpine
  pullPolicy: ` + tt.pullPolicy + `
steps:
  - name: step1
    command: echo test
`
			dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
			require.NoError(t, err)
			require.NotNil(t, dag.Container)
			assert.Equal(t, tt.expected, dag.Container.PullPolicy)
		})
	}

	// Error tests
	errorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "InvalidPullPolicy",
			yaml: `
container:
  image: alpine
  pullPolicy: invalid_policy
steps:
  - name: step1
    command: echo test
`,
			errContains: "failed to parse pull policy",
		},
		{
			name: "ContainerWithoutImage",
			yaml: `
container:
  pullPolicy: always
  env:
    - FOO: bar
steps:
  - name: step1
    command: echo test
`,
			errContains: "image is required when container is specified",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}

	// Complex container with all fields (separate due to many assertions)
	t.Run("ContainerWithAllFields", func(t *testing.T) {
		t.Parallel()
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
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
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
	t.Run("ExplicitAbsoluteWorkingDir", func(t *testing.T) {
		tempDir := t.TempDir()
		yaml := fmt.Sprintf(`
workingDir: %s
steps:
  - echo hello
`, tempDir)
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.Equal(t, tempDir, dag.WorkingDir)
	})

	t.Run("WorkingDirWithEnvVarExpansion", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("TEST_DIR", tempDir)
		yaml := `
workingDir: ${TEST_DIR}/subdir
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.Equal(t, filepath.Join(tempDir, "subdir"), dag.WorkingDir)
	})

	t.Run("DefaultWorkingDirWhenNoFile", func(t *testing.T) {
		yaml := `
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Should be current working directory when loaded from YAML (no file)
		expectedDir, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, expectedDir, dag.WorkingDir)
	})

	t.Run("RelativeWorkingDirResolvesAgainstDAGFile", func(t *testing.T) {
		// Create a temp directory with a DAG file
		tempDir := t.TempDir()
		dagFile := filepath.Join(tempDir, "dag.yaml")
		subDir := filepath.Join(tempDir, "scripts")

		yaml := `
workingDir: ./scripts
steps:
  - echo hello
`
		require.NoError(t, os.WriteFile(dagFile, []byte(yaml), 0644))

		dag, err := spec.Load(context.Background(), dagFile)
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Relative path should resolve against DAG file directory
		assert.Equal(t, subDir, dag.WorkingDir)
	})

	t.Run("RelativeWorkingDirWithoutDAGFile_ResolvesAgainstCWD", func(t *testing.T) {
		yaml := `
workingDir: ./subdir
steps:
  - echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag)

		// When no DAG file, relative path is resolved via fileutil.ResolvePathOrBlank
		// which uses filepath.Abs (resolves against CWD)
		cwd, err := os.Getwd()
		require.NoError(t, err)
		expectedDir := filepath.Join(cwd, "subdir")
		assert.Equal(t, expectedDir, dag.WorkingDir)
	})
}

func TestBuildStepWorkingDir(t *testing.T) {
	t.Run("StepWithDirField", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    dir: /tmp/mydir
    command: echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "/tmp/mydir", dag.Steps[0].Dir)
	})

	t.Run("StepWithWorkingDirField", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    workingDir: /tmp/myworkdir
    command: echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "/tmp/myworkdir", dag.Steps[0].Dir)
	})

	t.Run("StepWorkingDirTakesPrecedenceOverDir", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    dir: /tmp/dir
    workingDir: /tmp/workingdir
    command: echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		// workingDir should take precedence over dir
		assert.Equal(t, "/tmp/workingdir", dag.Steps[0].Dir)
	})

	t.Run("StepWithRelativeDir", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    dir: ./scripts
    command: echo hello
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		// At build time, relative dir is stored as-is (resolved at runtime)
		assert.Equal(t, "./scripts", dag.Steps[0].Dir)
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

		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Load environment variables from dotenv file
		dag.LoadDotEnv(context.Background())

		// Verify environment variables are in dag.Env (not process env)
		// Child processes will receive them via cmd.Env = AllEnvs()
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				envMap[key] = value
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
		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// LoadDotEnv should not fail even if dotenv file doesn't exist
		dag.LoadDotEnv(context.Background())

		// Environment variables from env should still be in dag.Env
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				envMap[key] = value
			}
		}
		assert.Equal(t, "test_value", envMap["TEST_VAR_LOAD_ENV"])
	})
}

func TestSecrets(t *testing.T) {
	t.Parallel()

	// Success tests
	successTests := []struct {
		name         string
		yaml         string
		wantCount    int
		wantEmpty    bool
		wantNil      bool
		checkSecrets func(t *testing.T, secrets []core.SecretRef)
	}{
		{
			name: "ValidSecretsArray",
			yaml: `
secrets:
  - name: DB_PASSWORD
    provider: gcp-secrets
    key: secret/data/prod/db
    options:
      namespace: production
  - name: API_KEY
    provider: env
    key: API_KEY
steps:
  - name: test
    command: echo "test"
`,
			wantCount: 2,
			checkSecrets: func(t *testing.T, secrets []core.SecretRef) {
				assert.Equal(t, "DB_PASSWORD", secrets[0].Name)
				assert.Equal(t, "gcp-secrets", secrets[0].Provider)
				assert.Equal(t, "secret/data/prod/db", secrets[0].Key)
				assert.Equal(t, "production", secrets[0].Options["namespace"])
				assert.Equal(t, "API_KEY", secrets[1].Name)
				assert.Equal(t, "env", secrets[1].Provider)
				assert.Equal(t, "API_KEY", secrets[1].Key)
				assert.Empty(t, secrets[1].Options)
			},
		},
		{
			name: "MinimalValidSecret",
			yaml: `
secrets:
  - name: MY_SECRET
    provider: env
    key: MY_SECRET
steps:
  - name: test
    command: echo "test"
`,
			wantCount: 1,
			checkSecrets: func(t *testing.T, secrets []core.SecretRef) {
				assert.Equal(t, "MY_SECRET", secrets[0].Name)
				assert.Equal(t, "env", secrets[0].Provider)
				assert.Equal(t, "MY_SECRET", secrets[0].Key)
			},
		},
		{
			name: "EmptySecretsArray",
			yaml: `
secrets: []
steps:
  - name: test
    command: echo "test"
`,
			wantEmpty: true,
		},
		{
			name: "NoSecretsField",
			yaml: `
steps:
  - name: test
    command: echo "test"
`,
			wantNil: true,
		},
		{
			name: "ComplexProviderOptions",
			yaml: `
secrets:
  - name: DB_PASSWORD
    provider: gcp-secrets
    key: projects/my-project/secrets/db-password/versions/latest
    options:
      projectId: my-project
      timeout: "30s"
      retries: "3"
steps:
  - name: test
    command: echo "test"
`,
			wantCount: 1,
			checkSecrets: func(t *testing.T, secrets []core.SecretRef) {
				assert.Equal(t, "DB_PASSWORD", secrets[0].Name)
				assert.Equal(t, "gcp-secrets", secrets[0].Provider)
				assert.Equal(t, "projects/my-project/secrets/db-password/versions/latest", secrets[0].Key)
				assert.Equal(t, "my-project", secrets[0].Options["projectId"])
				assert.Equal(t, "30s", secrets[0].Options["timeout"])
				assert.Equal(t, "3", secrets[0].Options["retries"])
			},
		},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, dag.Secrets)
			} else if tt.wantEmpty {
				assert.Empty(t, dag.Secrets)
			} else {
				require.Len(t, dag.Secrets, tt.wantCount)
				if tt.checkSecrets != nil {
					tt.checkSecrets(t, dag.Secrets)
				}
			}
		})
	}

	// Error tests
	errorTests := []struct {
		name         string
		yaml         string
		errContains  []string
	}{
		{
			name: "MissingNameField",
			yaml: `
secrets:
  - provider: vault
    key: secret/data/test
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"'name' field is required"},
		},
		{
			name: "MissingProviderField",
			yaml: `
secrets:
  - name: MY_SECRET
    key: secret/data/test
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"'provider' field is required"},
		},
		{
			name: "MissingKeyField",
			yaml: `
secrets:
  - name: MY_SECRET
    provider: vault
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"'key' field is required"},
		},
		{
			name: "DuplicateSecretNames",
			yaml: `
secrets:
  - name: API_KEY
    provider: vault
    key: secret/v1
  - name: API_KEY
    provider: env
    key: API_KEY
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"duplicate secret name", "API_KEY"},
		},
		{
			name: "InvalidSecretsType",
			yaml: `
secrets: "invalid string"
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"Secrets", "array or slice"},
		},
		{
			name: "InvalidSecretItemType",
			yaml: `
secrets:
  - "invalid string item"
steps:
  - name: test
    command: echo "test"
`,
			errContains: []string{"Secrets", "map or struct"},
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			for _, contains := range tt.errContains {
				assert.Contains(t, err.Error(), contains)
			}
		})
	}
}

func TestBuildStepParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		wantEmpty   bool
		wantParams  map[string]string
	}{
		{
			name: "ParamsAsMap",
			yaml: `
steps:
  - name: test
    command: actions/checkout@v4
    executor:
      type: github_action
    params:
      repository: myorg/myrepo
      ref: main
      token: secret123
`,
			wantParams: map[string]string{
				"repository": "myorg/myrepo",
				"ref":        "main",
				"token":      "secret123",
			},
		},
		{
			name: "ParamsAsString",
			yaml: `
steps:
  - name: test
    command: actions/setup-go@v5
    executor:
      type: github_action
    params: go-version=1.21 cache=true
`,
			wantParams: map[string]string{
				"go-version": "1.21",
				"cache":      "true",
			},
		},
		{
			name: "ParamsWithNumbers",
			yaml: `
steps:
  - name: test
    command: some-action
    params:
      timeout: 300
      retries: 3
      enabled: true
`,
			wantParams: map[string]string{
				"timeout": "300",
				"retries": "3",
				"enabled": "true",
			},
		},
		{
			name: "NoParams",
			yaml: `
steps:
  - name: test
    command: echo hello
`,
			wantEmpty: true,
		},
		{
			name: "EmptyParams",
			yaml: `
steps:
  - name: test
    command: echo hello
    params: {}
`,
			wantParams: map[string]string{},
		},
		{
			name: "ParamsWithQuotedValues",
			yaml: `
steps:
  - name: test
    command: some-action
    params: message="hello world" count="42"
`,
			wantParams: map[string]string{
				"message": "hello world",
				"count":   "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)

			step := dag.Steps[0]
			if tt.wantEmpty {
				assert.True(t, step.Params.IsEmpty())
				return
			}

			params, err := step.Params.AsStringMap()
			require.NoError(t, err)
			if len(tt.wantParams) == 0 {
				assert.Empty(t, params)
			} else {
				for k, v := range tt.wantParams {
					assert.Equal(t, v, params[k])
				}
			}
		})
	}
}

func TestBuildShell(t *testing.T) {
	// Standard shell configuration tests
	tests := []struct {
		name          string
		yaml          string
		wantShell     string
		wantShellArgs []string
		wantNotEmpty  bool // For default shell tests
	}{
		{
			name: "SimpleString",
			yaml: `
shell: bash
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: nil,
		},
		{
			name: "StringWithArgs",
			yaml: `
shell: bash -e
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: []string{"-e"},
		},
		{
			name: "StringWithMultipleArgs",
			yaml: `
shell: bash -e -u -o pipefail
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: []string{"-e", "-u", "-o", "pipefail"},
		},
		{
			name: "Array",
			yaml: `
shell:
  - bash
  - -e
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: []string{"-e"},
		},
		{
			name: "ArrayWithMultipleArgs",
			yaml: `
shell:
  - bash
  - -e
  - -u
  - -o
  - pipefail
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: []string{"-e", "-u", "-o", "pipefail"},
		},
		{
			name: "NotSpecified",
			yaml: `
steps:
  - "echo hello"
`,
			wantNotEmpty: true,
		},
		{
			name: "EmptyString",
			yaml: `
shell: ""
steps:
  - "echo hello"
`,
			wantNotEmpty: true,
		},
		{
			name: "EmptyArray",
			yaml: `
shell: []
steps:
  - "echo hello"
`,
			wantNotEmpty: true,
		},
		{
			name: "Pwsh",
			yaml: `
shell: pwsh
steps:
  - "Write-Output hello"
`,
			wantShell:     "pwsh",
			wantShellArgs: nil,
		},
		{
			name: "PwshWithArgs",
			yaml: `
shell: pwsh -NoProfile -NonInteractive
steps:
  - "Write-Output hello"
`,
			wantShell:     "pwsh",
			wantShellArgs: []string{"-NoProfile", "-NonInteractive"},
		},
		{
			name: "WithQuotedArgs",
			yaml: `
shell: bash -c "set -e"
steps:
  - "echo hello"
`,
			wantShell:     "bash",
			wantShellArgs: []string{"-c", "set -e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			if tt.wantNotEmpty {
				assert.NotEmpty(t, dag.Shell)
			} else {
				assert.Equal(t, tt.wantShell, dag.Shell)
			}
			if tt.wantShellArgs == nil {
				assert.Empty(t, dag.ShellArgs)
			} else {
				assert.Equal(t, tt.wantShellArgs, dag.ShellArgs)
			}
		})
	}

	// Environment variable tests (cannot use t.Parallel due to t.Setenv)
	t.Run("WithEnvVar", func(t *testing.T) {
		t.Setenv("MY_SHELL", "/bin/zsh")
		data := []byte(`
shell: $MY_SHELL
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, "/bin/zsh", dag.Shell)
		assert.Empty(t, dag.ShellArgs)
	})

	t.Run("ArrayWithEnvVar", func(t *testing.T) {
		t.Setenv("SHELL_ARG", "-x")
		data := []byte(`
shell:
  - bash
  - $SHELL_ARG
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, "bash", dag.Shell)
		assert.Equal(t, []string{"-x"}, dag.ShellArgs)
	})

	// NoEval tests (cannot use t.Parallel due to t.Setenv)
	t.Run("NoEvalPreservesRaw", func(t *testing.T) {
		t.Setenv("MY_SHELL", "/bin/zsh")
		data := []byte(`
shell: $MY_SHELL -e
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		assert.Equal(t, "$MY_SHELL", dag.Shell)
		assert.Equal(t, []string{"-e"}, dag.ShellArgs)
	})

	t.Run("ArrayNoEvalPreservesRaw", func(t *testing.T) {
		t.Setenv("SHELL_ARG", "-x")
		data := []byte(`
shell:
  - bash
  - $SHELL_ARG
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		assert.Equal(t, "bash", dag.Shell)
		assert.Equal(t, []string{"$SHELL_ARG"}, dag.ShellArgs)
	})
}

func TestBuildStepShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		yaml              string
		wantDAGShell      string
		wantDAGShellArgs  []string
		wantStepShell     string
		wantStepShellArgs []string
		wantStepEmpty     bool
	}{
		{
			name: "SimpleString",
			yaml: `
steps:
  - name: test
    shell: zsh
    command: echo hello
`,
			wantStepShell:     "zsh",
			wantStepShellArgs: nil,
		},
		{
			name: "StringWithArgs",
			yaml: `
steps:
  - name: test
    shell: bash -e -u
    command: echo hello
`,
			wantStepShell:     "bash",
			wantStepShellArgs: []string{"-e", "-u"},
		},
		{
			name: "Array",
			yaml: `
steps:
  - name: test
    shell:
      - bash
      - -e
      - -o
      - pipefail
    command: echo hello
`,
			wantStepShell:     "bash",
			wantStepShellArgs: []string{"-e", "-o", "pipefail"},
		},
		{
			name: "OverridesDAGShell",
			yaml: `
shell: bash -e
steps:
  - name: test
    shell: zsh
    command: echo hello
`,
			wantDAGShell:      "bash",
			wantDAGShellArgs:  []string{"-e"},
			wantStepShell:     "zsh",
			wantStepShellArgs: nil,
		},
		{
			name: "NotSpecified",
			yaml: `
steps:
  - name: test
    command: echo hello
`,
			wantStepEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)

			if tt.wantDAGShell != "" {
				assert.Equal(t, tt.wantDAGShell, dag.Shell)
				assert.Equal(t, tt.wantDAGShellArgs, dag.ShellArgs)
			}

			if tt.wantStepEmpty {
				assert.Empty(t, dag.Steps[0].Shell)
			} else {
				assert.Equal(t, tt.wantStepShell, dag.Steps[0].Shell)
			}

			if tt.wantStepShellArgs == nil {
				assert.Empty(t, dag.Steps[0].ShellArgs)
			} else {
				assert.Equal(t, tt.wantStepShellArgs, dag.Steps[0].ShellArgs)
			}
		})
	}
}

func TestBuildStepTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		wantTimeout time.Duration
		wantErr     string
	}{
		{
			name: "Positive",
			yaml: `
steps:
  - name: work
    command: echo doing
    timeoutSec: 5
`,
			wantTimeout: 5 * time.Second,
		},
		{
			name: "ZeroExplicit",
			yaml: `
steps:
  - name: work
    command: echo none
    timeoutSec: 0
`,
			wantTimeout: 0,
		},
		{
			name: "ZeroOmitted",
			yaml: `
steps:
  - name: work
    command: echo omitted
`,
			wantTimeout: 0,
		},
		{
			name: "Negative",
			yaml: `
steps:
  - name: bad
    command: echo fail
    timeoutSec: -3
`,
			wantErr: "timeoutSec must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			assert.Equal(t, tt.wantTimeout, dag.Steps[0].Timeout)
		})
	}
}

func TestBuildHandlers(t *testing.T) {
	t.Parallel()

	t.Run("AllHandlers", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
handlerOn:
  init:
    command: echo init
  exit:
    command: echo exit
  success:
    command: echo success
  failure:
    command: echo failure
  abort:
    command: echo abort
steps:
  - name: main
    command: echo main
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.HandlerOn.Init)
		assert.Equal(t, "echo init", dag.HandlerOn.Init.CmdWithArgs)
		require.NotNil(t, dag.HandlerOn.Exit)
		assert.Equal(t, "echo exit", dag.HandlerOn.Exit.CmdWithArgs)
		require.NotNil(t, dag.HandlerOn.Success)
		assert.Equal(t, "echo success", dag.HandlerOn.Success.CmdWithArgs)
		require.NotNil(t, dag.HandlerOn.Failure)
		assert.Equal(t, "echo failure", dag.HandlerOn.Failure.CmdWithArgs)
		require.NotNil(t, dag.HandlerOn.Cancel)
		assert.Equal(t, "echo abort", dag.HandlerOn.Cancel.CmdWithArgs)
	})

	t.Run("CancelDeprecated", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
handlerOn:
  cancel:
    command: echo cancel
steps:
  - name: main
    command: echo main
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.HandlerOn.Cancel)
		assert.Equal(t, "echo cancel", dag.HandlerOn.Cancel.CmdWithArgs)
	})

	t.Run("AbortAndCancelConflict", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
handlerOn:
  abort:
    command: echo abort
  cancel:
    command: echo cancel
steps:
  - name: main
    command: echo main
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both 'abort' and 'cancel'")
	})
}

func TestBuildParallel(t *testing.T) {
	t.Parallel()

	t.Run("StringForm_VariableReference", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel: ${ITEMS}
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		assert.Equal(t, "${ITEMS}", dag.Steps[0].Parallel.Variable)
		assert.Equal(t, core.DefaultMaxConcurrent, dag.Steps[0].Parallel.MaxConcurrent)
	})

	t.Run("ArrayForm_StaticItems", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - item1
      - item2
      - item3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 3)
		assert.Equal(t, "item1", dag.Steps[0].Parallel.Items[0].Value)
		assert.Equal(t, "item2", dag.Steps[0].Parallel.Items[1].Value)
		assert.Equal(t, "item3", dag.Steps[0].Parallel.Items[2].Value)
	})

	t.Run("ArrayForm_NumericItems", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - 1
      - 2
      - 3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 3)
		assert.Equal(t, "1", dag.Steps[0].Parallel.Items[0].Value)
		assert.Equal(t, "2", dag.Steps[0].Parallel.Items[1].Value)
		assert.Equal(t, "3", dag.Steps[0].Parallel.Items[2].Value)
	})

	t.Run("ArrayForm_ObjectItems", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - name: first
        value: 100
      - name: second
        value: 200
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 2)
		assert.Equal(t, "first", dag.Steps[0].Parallel.Items[0].Params["name"])
		assert.Equal(t, "100", dag.Steps[0].Parallel.Items[0].Params["value"])
		assert.Equal(t, "second", dag.Steps[0].Parallel.Items[1].Params["name"])
		assert.Equal(t, "200", dag.Steps[0].Parallel.Items[1].Params["value"])
	})

	t.Run("ObjectForm_WithVariable", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items: ${MY_ITEMS}
      maxConcurrent: 5
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		assert.Equal(t, "${MY_ITEMS}", dag.Steps[0].Parallel.Variable)
		assert.Equal(t, 5, dag.Steps[0].Parallel.MaxConcurrent)
	})

	t.Run("ObjectForm_WithStaticItems", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items:
        - a
        - b
      maxConcurrent: 3
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 2)
		assert.Equal(t, "a", dag.Steps[0].Parallel.Items[0].Value)
		assert.Equal(t, "b", dag.Steps[0].Parallel.Items[1].Value)
		assert.Equal(t, 3, dag.Steps[0].Parallel.MaxConcurrent)
	})

	t.Run("ObjectForm_MaxConcurrentAsInt64", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items: ${ITEMS}
      maxConcurrent: 10
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		assert.Equal(t, 10, dag.Steps[0].Parallel.MaxConcurrent)
	})

	t.Run("ObjectForm_MaxConcurrentAsFloat", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items: ${ITEMS}
      maxConcurrent: 7.0
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		assert.Equal(t, 7, dag.Steps[0].Parallel.MaxConcurrent)
	})

	t.Run("Error_InvalidParallelType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel: 123
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel must be string, array, or object")
	})

	t.Run("Error_InvalidItemsType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items: 123
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel.items must be string or array")
	})

	t.Run("Error_InvalidMaxConcurrentType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      items: ${ITEMS}
      maxConcurrent: "invalid"
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel.maxConcurrent must be int")
	})

	t.Run("Error_InvalidItemValue", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - [nested, array]
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel items must be strings, numbers, or objects")
	})

	t.Run("ObjectItem_WithBoolValue", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - name: test
        enabled: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 1)
		assert.Equal(t, "true", dag.Steps[0].Parallel.Items[0].Params["enabled"])
	})

	t.Run("ObjectItem_WithFloatValue", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - name: test
        rate: 3.14
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		require.NotNil(t, dag.Steps[0].Parallel)
		require.Len(t, dag.Steps[0].Parallel.Items, 1)
		assert.Equal(t, "3.14", dag.Steps[0].Parallel.Items[0].Params["rate"])
	})

	t.Run("Error_InvalidObjectParamType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - name: test
        nested:
          key: value
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parameter values must be strings, numbers, or booleans")
	})

	t.Run("ParallelSetsExecutorType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: parallel-step
    call: subdag
    parallel:
      - item1
      - item2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, core.ExecutorTypeParallel, dag.Steps[0].ExecutorConfig.Type)
	})
}

func TestBuildRegistryAuthsExtra(t *testing.T) {
	t.Parallel()

	t.Run("StringForm_JSON", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
registryAuths: '{"auths":{"docker.io":{"auth":"dXNlcjpwYXNz"}}}'
steps:
  - name: test
    command: echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Contains(t, dag.RegistryAuths, "_json")
		assert.Equal(t, `{"auths":{"docker.io":{"auth":"dXNlcjpwYXNz"}}}`, dag.RegistryAuths["_json"].Auth)
	})

	t.Run("MapForm_WithAuthString", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
registryAuths:
  docker.io: "dXNlcjpwYXNz"
  gcr.io: "another-token"
steps:
  - name: test
    command: echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Contains(t, dag.RegistryAuths, "docker.io")
		require.Contains(t, dag.RegistryAuths, "gcr.io")
		assert.Equal(t, "dXNlcjpwYXNz", dag.RegistryAuths["docker.io"].Auth)
		assert.Equal(t, "another-token", dag.RegistryAuths["gcr.io"].Auth)
	})

	t.Run("Error_InvalidType", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
registryAuths: 123
steps:
  - name: test
    command: echo test
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type")
	})
}

func TestLoadWithOptions(t *testing.T) {
	t.Run("WithoutEval_DisablesEnvExpansion", func(t *testing.T) {
		// Cannot use t.Parallel() with t.Setenv()
		t.Setenv("MY_VAR", "expanded-value")

		data := []byte(`
env:
  - TEST: "${MY_VAR}"
steps:
  - name: test
    command: echo test
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)

		// When NoEval is set, the variable should not be expanded
		// dag.Env is []string in format "KEY=VALUE"
		assert.Contains(t, dag.Env, "TEST=${MY_VAR}")
	})

	t.Run("WithAllowBuildErrors_CapturesErrors", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: test
    command: echo test
    depends:
      - nonexistent
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagAllowBuildErrors})
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.NotEmpty(t, dag.BuildErrors)
	})

	t.Run("SkipSchemaValidation_SkipsParamsSchema", func(t *testing.T) {
		t.Parallel()
		// BuildFlagSkipSchemaValidation skips JSON schema validation for params,
		// not YAML structure validation
		data := []byte(`
params:
  schema: "nonexistent-schema.json"
  values:
    key: value
steps:
  - name: test
    command: echo test
`)
		// Without the flag, it would error due to missing schema file
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)

		// With schema validation skipped, it succeeds
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagSkipSchemaValidation})
		require.NoError(t, err)
		require.NotNil(t, dag)
	})
}
