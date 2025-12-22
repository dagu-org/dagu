package spec

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSchemaReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   any
		expected string
	}{
		{
			name:     "Nil",
			params:   nil,
			expected: "",
		},
		{
			name:     "NotAMap",
			params:   "string value",
			expected: "",
		},
		{
			name:     "EmptyMap",
			params:   map[string]any{},
			expected: "",
		},
		{
			name:     "NoSchemaKey",
			params:   map[string]any{"values": map[string]any{"foo": "bar"}},
			expected: "",
		},
		{
			name:     "SchemaKeyNotString",
			params:   map[string]any{"schema": 123},
			expected: "",
		},
		{
			name:     "SchemaKeyIsMap",
			params:   map[string]any{"schema": map[string]any{"type": "object"}},
			expected: "",
		},
		{
			name:     "ValidSchemaReference",
			params:   map[string]any{"schema": "schema.json"},
			expected: "schema.json",
		},
		{
			name:     "ValidSchemaReferenceWithValues",
			params:   map[string]any{"schema": "./schemas/params.json", "values": map[string]any{"foo": "bar"}},
			expected: "./schemas/params.json",
		},
		{
			name:     "HTTPSchemaReference",
			params:   map[string]any{"schema": "https://example.com/schema.json"},
			expected: "https://example.com/schema.json",
		},
		{
			name:     "EmptySchemaString",
			params:   map[string]any{"schema": ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractSchemaReference(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadSchemaFromURL(t *testing.T) {
	t.Parallel()

	t.Run("SuccessfulLoad", func(t *testing.T) {
		t.Parallel()

		schemaContent := `{"type": "object", "properties": {"foo": {"type": "string"}}}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(schemaContent))
		}))
		defer server.Close()

		data, err := loadSchemaFromURL(server.URL + "/schema.json")
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})

	t.Run("HTTPError", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := loadSchemaFromURL(server.URL + "/missing.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("ServerError", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := loadSchemaFromURL(server.URL + "/schema.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		t.Parallel()

		_, err := loadSchemaFromURL("://invalid-url")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("UnsupportedScheme", func(t *testing.T) {
		t.Parallel()

		_, err := loadSchemaFromURL("ftp://example.com/schema.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported URL scheme")
	})

	t.Run("ConnectionRefused", func(t *testing.T) {
		t.Parallel()

		// Use a port that's unlikely to be in use
		_, err := loadSchemaFromURL("http://127.0.0.1:59999/schema.json")
		require.Error(t, err)
	})
}

func TestLoadSchemaFromFile(t *testing.T) {
	t.Parallel()

	schemaContent := `{"type": "object"}`

	t.Run("AbsolutePath", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		schemaPath := filepath.Join(tmpDir, "schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(schemaContent), 0600))

		data, err := loadSchemaFromFile("", "", schemaPath)
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})

	t.Run("FromWorkingDir", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		schemaPath := filepath.Join(workingDir, "schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(schemaContent), 0600))

		data, err := loadSchemaFromFile(workingDir, "", "schema.json")
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})

	t.Run("FromDAGDirectory", func(t *testing.T) {
		t.Parallel()

		dagDir := t.TempDir()
		schemaPath := filepath.Join(dagDir, "schema.json")
		dagPath := filepath.Join(dagDir, "dag.yaml")
		require.NoError(t, os.WriteFile(schemaPath, []byte(schemaContent), 0600))

		data, err := loadSchemaFromFile("", dagPath, "schema.json")
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})

	t.Run("WorkingDirTakesPrecedenceOverDAGDir", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		dagDir := t.TempDir()

		wdSchema := `{"type": "object", "title": "working-dir"}`
		dagSchema := `{"type": "object", "title": "dag-dir"}`

		require.NoError(t, os.WriteFile(filepath.Join(workingDir, "schema.json"), []byte(wdSchema), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dagDir, "schema.json"), []byte(dagSchema), 0600))

		dagPath := filepath.Join(dagDir, "dag.yaml")
		data, err := loadSchemaFromFile(workingDir, dagPath, "schema.json")
		require.NoError(t, err)
		assert.Equal(t, wdSchema, string(data))
	})

	t.Run("FileNotFound", func(t *testing.T) {
		t.Parallel()

		_, err := loadSchemaFromFile("", "", "nonexistent-schema.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("FileNotFoundWithWorkingDir", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		_, err := loadSchemaFromFile(workingDir, "", "nonexistent.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("FileNotFoundWithDAGDir", func(t *testing.T) {
		t.Parallel()

		dagDir := t.TempDir()
		dagPath := filepath.Join(dagDir, "dag.yaml")
		_, err := loadSchemaFromFile("", dagPath, "nonexistent.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("SubdirectoryPath", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		schemasDir := filepath.Join(workingDir, "schemas")
		require.NoError(t, os.MkdirAll(schemasDir, 0755))
		schemaPath := filepath.Join(schemasDir, "params.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(schemaContent), 0600))

		data, err := loadSchemaFromFile(workingDir, "", "schemas/params.json")
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})

	t.Run("EmptyWorkingDirAndDAGLocation", func(t *testing.T) {
		t.Parallel()

		_, err := loadSchemaFromFile("", "", "schema.json")
		require.Error(t, err)
	})

	t.Run("WhitespaceOnlyWorkingDir", func(t *testing.T) {
		t.Parallel()

		dagDir := t.TempDir()
		schemaPath := filepath.Join(dagDir, "schema.json")
		dagPath := filepath.Join(dagDir, "dag.yaml")
		require.NoError(t, os.WriteFile(schemaPath, []byte(schemaContent), 0600))

		// Whitespace-only workingDir should be skipped, fall back to dagDir
		data, err := loadSchemaFromFile("   ", dagPath, "schema.json")
		require.NoError(t, err)
		assert.Equal(t, schemaContent, string(data))
	})
}

func TestGetSchemaFromRef(t *testing.T) {
	t.Parallel()

	validSchemaContent := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`

	t.Run("LocalFileSchema", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		schemaPath := filepath.Join(tmpDir, "schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(validSchemaContent), 0600))

		resolved, err := getSchemaFromRef("", "", schemaPath)
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})

	t.Run("RemoteURLSchema", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(validSchemaContent))
		}))
		defer server.Close()

		resolved, err := getSchemaFromRef("", "", server.URL+"/schema.json")
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})

	t.Run("HTTPSSchemaReference", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(validSchemaContent))
		}))
		defer server.Close()

		// This will fail due to self-signed cert, but tests the https:// detection
		_, err := getSchemaFromRef("", "", server.URL+"/schema.json")
		// We expect an error due to certificate verification
		require.Error(t, err)
	})

	t.Run("InvalidJSONSchema", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		schemaPath := filepath.Join(tmpDir, "invalid.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte("not valid json"), 0600))

		_, err := getSchemaFromRef("", "", schemaPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse schema JSON")
	})

	t.Run("SchemaFileNotFound", func(t *testing.T) {
		t.Parallel()

		_, err := getSchemaFromRef("", "", "nonexistent.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load schema")
	})

	t.Run("URLNotFound", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := getSchemaFromRef("", "", server.URL+"/missing.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load schema")
	})
}

func TestResolveSchemaFromParams(t *testing.T) {
	t.Parallel()

	validSchemaContent := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"batch_size": {"type": "integer", "default": 10}
		}
	}`

	t.Run("NoSchemaReference", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveSchemaFromParams(nil, "", "")
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("ParamsNotMap", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveSchemaFromParams("string params", "", "")
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("ParamsWithoutSchema", func(t *testing.T) {
		t.Parallel()

		params := map[string]any{
			"values": map[string]any{"foo": "bar"},
		}
		resolved, err := resolveSchemaFromParams(params, "", "")
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("ParamsWithValidSchema", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		schemaPath := filepath.Join(tmpDir, "schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(validSchemaContent), 0600))

		params := map[string]any{
			"schema": schemaPath,
			"values": map[string]any{"batch_size": 20},
		}
		resolved, err := resolveSchemaFromParams(params, "", "")
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})

	t.Run("ParamsWithInvalidSchemaPath", func(t *testing.T) {
		t.Parallel()

		params := map[string]any{
			"schema": "nonexistent.json",
		}
		_, err := resolveSchemaFromParams(params, "", "")
		require.Error(t, err)
	})

	t.Run("ParamsWithRemoteSchema", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(validSchemaContent))
		}))
		defer server.Close()

		params := map[string]any{
			"schema": server.URL + "/schema.json",
		}
		resolved, err := resolveSchemaFromParams(params, "", "")
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})

	t.Run("UsesWorkingDirForResolution", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		schemaPath := filepath.Join(workingDir, "schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(validSchemaContent), 0600))

		params := map[string]any{
			"schema": "schema.json",
		}
		resolved, err := resolveSchemaFromParams(params, workingDir, "")
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})

	t.Run("UsesDAGLocationForResolution", func(t *testing.T) {
		t.Parallel()

		dagDir := t.TempDir()
		schemaPath := filepath.Join(dagDir, "schema.json")
		dagPath := filepath.Join(dagDir, "dag.yaml")
		require.NoError(t, os.WriteFile(schemaPath, []byte(validSchemaContent), 0600))

		params := map[string]any{
			"schema": "schema.json",
		}
		resolved, err := resolveSchemaFromParams(params, "", dagPath)
		require.NoError(t, err)
		assert.NotNil(t, resolved)
	})
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(schemaContent)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 25
    environment: "staging"
`, tmpFile.Name()))

	dag, err := LoadYAML(context.Background(), data)
	require.NoError(t, err)

	require.Len(t, dag.Params, 2)
	require.Contains(t, dag.Params, "batch_size=25")
	require.Contains(t, dag.Params, "environment=staging")
}

func TestBuildParamsWithRemoteSchemaReference(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/schemas/dag-params.json", func(w http.ResponseWriter, _ *http.Request) {
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
		_, _ = w.Write([]byte(schemaContent))
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

	dag, err := LoadYAML(context.Background(), data)
	require.NoError(t, err)

	require.Len(t, dag.Params, 2)
	require.Contains(t, dag.Params, "batch_size=50")
	require.Contains(t, dag.Params, "environment=prod")
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

		dag, err := LoadYAML(context.Background(), data)
		require.NoError(t, err)

		require.Contains(t, dag.Params, "batch_size=42")
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

		dag, err := Load(context.Background(), dagPath)
		require.NoError(t, err)

		require.Contains(t, dag.Params, "batch_size=7")
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
		defer func() { _ = os.Chdir(orig) }()

		data := []byte(fmt.Sprintf(`
workingDir: %s
params:
  schema: "schema.json"
  values:
    environment: "dev"
`, wd))

		dag, err := LoadYAML(context.Background(), data)
		require.NoError(t, err)

		require.Contains(t, dag.Params, "batch_size=99")
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
		_, err := LoadYAML(context.Background(), data)
		require.Error(t, err)

		dag, err := LoadYAMLWithOpts(context.Background(), data, BuildOpts{
			Flags: BuildFlagSkipSchemaValidation,
		})
		require.NoError(t, err)

		require.Len(t, dag.Params, 1)
		require.Contains(t, dag.Params, "foo=bar")
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
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		require.NoError(t, tmpFile.Close())

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
`, tmpFile.Name()))

		cliParams := "batch_size=100 environment=prod"
		_, err = LoadYAML(context.Background(), data, WithParams(cliParams))
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
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		require.NoError(t, tmpFile.Close())

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 75
`, tmpFile.Name()))

		dag, err := LoadYAML(context.Background(), data)
		require.NoError(t, err)

		require.Len(t, dag.Params, 3)
		require.Contains(t, dag.Params, "batch_size=75")
		require.Contains(t, dag.Params, "environment=development")
		require.Contains(t, dag.Params, "debug=true")
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
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		_, err = tmpFile.WriteString(schemaContent)
		require.NoError(t, err)
		require.NoError(t, tmpFile.Close())

		data := []byte(fmt.Sprintf(`
params:
  schema: "%s"
  values:
    batch_size: 50
    environment: "production"
    debug: false
    timeout: 600
`, tmpFile.Name()))

		dag, err := LoadYAML(context.Background(), data)
		require.NoError(t, err)

		require.Len(t, dag.Params, 4)
		require.Contains(t, dag.Params, "batch_size=50")
		require.Contains(t, dag.Params, "environment=production")
		require.Contains(t, dag.Params, "debug=false")
		require.Contains(t, dag.Params, "timeout=600")
	})
}
