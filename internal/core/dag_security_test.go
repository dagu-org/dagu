package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDAGJSONSecuritySensitiveFieldsExcluded verifies that sensitive fields
// with json:"-" tags are NOT serialized to JSON, preventing secret leakage.
func TestDAGJSONSecuritySensitiveFieldsExcluded(t *testing.T) {
	dag := &DAG{
		Name:        "test-dag",
		Env:         []string{"SECRET_KEY=super_secret_value", "API_TOKEN=my_token"},
		Params:      []string{"password=mypassword"},
		ParamsJSON:  `{"password":"mypassword"}`,
		Shell:       "/bin/bash",
		ShellArgs:   []string{"-c"},
		WorkingDir:  "/secret/path",
		RegistryAuths: map[string]*AuthConfig{
			"docker.io": {
				Username: "user",
				Password: "docker_secret_password",
			},
		},
		YamlData: []byte("name: test-dag"),
	}

	data, err := json.MarshalIndent(dag, "", "  ")
	require.NoError(t, err)

	jsonStr := string(data)

	// Verify sensitive fields are NOT in JSON
	sensitiveFields := map[string]string{
		"Env":           "super_secret_value",
		"Params":        "mypassword",
		"ParamsJSON":    `"password"`,
		"Shell":         "/bin/bash",
		"ShellArgs":     `"-c"`,
		"WorkingDir":    "/secret/path",
		"RegistryAuths": "docker_secret_password",
	}

	for field, secret := range sensitiveFields {
		assert.NotContains(t, jsonStr, secret,
			"SECURITY FAILURE: %s contains secret '%s' in JSON output", field, secret)
	}

	// Verify safe fields ARE in JSON
	assert.Contains(t, jsonStr, "test-dag", "name should be preserved in JSON")
	assert.Contains(t, jsonStr, "yamlData", "yamlData should be preserved in JSON")
}

// TestContainerJSONSecurityEnvExcluded verifies that Container.Env is excluded from JSON.
func TestContainerJSONSecurityEnvExcluded(t *testing.T) {
	container := &Container{
		Image: "nginx:latest",
		Env:   []string{"DB_PASSWORD=super_secret", "API_KEY=my_api_key"},
	}

	data, err := json.MarshalIndent(container, "", "  ")
	require.NoError(t, err)

	jsonStr := string(data)

	// Container.Env should NOT be in JSON
	assert.NotContains(t, jsonStr, "super_secret", "Container.Env should not contain secrets in JSON")
	assert.NotContains(t, jsonStr, "my_api_key", "Container.Env should not contain API keys in JSON")
	assert.NotContains(t, jsonStr, "DB_PASSWORD", "Container.Env key should not be in JSON")

	// Image should be preserved
	assert.Contains(t, jsonStr, "nginx:latest", "Container.Image should be preserved in JSON")
}

// TestDAGRoundTripMissingFields verifies that when loading a DAG from JSON,
// the excluded fields are empty (which triggers re-evaluation from YamlData).
func TestDAGRoundTripMissingFields(t *testing.T) {
	original := &DAG{
		Name:        "test-dag",
		Env:         []string{"SECRET_KEY=value"},
		Params:      []string{"param1=value1"},
		ParamsJSON:  `{"param1":"value1"}`,
		Shell:       "/bin/bash",
		ShellArgs:   []string{"-c"},
		WorkingDir:  "/work/dir",
		RegistryAuths: map[string]*AuthConfig{
			"docker.io": {Password: "secret"},
		},
		YamlData: []byte("name: test-dag\nenv:\n  SECRET_KEY: value"),
	}

	// Serialize to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize from JSON
	var loaded DAG
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// Sensitive fields should be empty after round-trip
	assert.Empty(t, loaded.Env, "Env should be empty after JSON round-trip")
	assert.Empty(t, loaded.Params, "Params should be empty after JSON round-trip")
	assert.Empty(t, loaded.ParamsJSON, "ParamsJSON should be empty after JSON round-trip")
	assert.Empty(t, loaded.Shell, "Shell should be empty after JSON round-trip")
	assert.Empty(t, loaded.ShellArgs, "ShellArgs should be empty after JSON round-trip")
	assert.Empty(t, loaded.WorkingDir, "WorkingDir should be empty after JSON round-trip")
	assert.Nil(t, loaded.RegistryAuths, "RegistryAuths should be nil after JSON round-trip")

	// Safe fields should be preserved
	assert.Equal(t, "test-dag", loaded.Name, "Name should be preserved")
	assert.NotEmpty(t, loaded.YamlData, "YamlData should be preserved")
}

// TestHasEvaluatedFields verifies the hasEvaluatedFields() helper works correctly.
func TestHasEvaluatedFields(t *testing.T) {
	tests := []struct {
		name     string
		dag      *DAG
		expected bool
	}{
		{
			name:     "empty DAG",
			dag:      &DAG{},
			expected: false,
		},
		{
			name:     "DAG with Env",
			dag:      &DAG{Env: []string{"KEY=value"}},
			expected: true,
		},
		{
			name:     "DAG with Params",
			dag:      &DAG{Params: []string{"param=value"}},
			expected: true,
		},
		{
			name:     "DAG with Shell",
			dag:      &DAG{Shell: "/bin/bash"},
			expected: true,
		},
		{
			name:     "DAG with WorkingDir",
			dag:      &DAG{WorkingDir: "/work"},
			expected: true,
		},
		{
			name:     "DAG with RegistryAuths",
			dag:      &DAG{RegistryAuths: map[string]*AuthConfig{"docker.io": {}}},
			expected: true,
		},
		{
			name:     "DAG with Container.Env",
			dag:      &DAG{Container: &Container{Env: []string{"KEY=value"}}},
			expected: true,
		},
		{
			name:     "DAG with only safe fields",
			dag:      &DAG{Name: "test", YamlData: []byte("test")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dag.hasEvaluatedFields()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestJSONFieldTagsPresent verifies all sensitive fields have json:"-" tag.
func TestJSONFieldTagsPresent(t *testing.T) {
	dag := &DAG{
		Env:           []string{"test"},
		Params:        []string{"test"},
		ParamsJSON:    "test",
		Shell:         "test",
		ShellArgs:     []string{"test"},
		WorkingDir:    "test",
		RegistryAuths: map[string]*AuthConfig{"test": {}},
	}

	data, err := json.Marshal(dag)
	require.NoError(t, err)

	jsonStr := string(data)

	// None of the field names should appear in JSON output
	sensitiveFieldNames := []string{
		`"env"`,
		`"params"`,
		`"paramsJSON"`,
		`"shell"`,
		`"shellArgs"`,
		`"workingDir"`,
		`"registryAuths"`,
	}

	for _, fieldName := range sensitiveFieldNames {
		if strings.Contains(jsonStr, fieldName) {
			t.Errorf("SECURITY: JSON output contains sensitive field %s: %s", fieldName, jsonStr)
		}
	}
}
