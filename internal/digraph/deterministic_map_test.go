package digraph

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicMap_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    DeterministicMap
		expected string
	}{
		{
			name:     "EmptyMap",
			input:    DeterministicMap{},
			expected: `{}`,
		},
		{
			name:     "NilMap",
			input:    nil,
			expected: `null`,
		},
		{
			name: "SingleKey",
			input: DeterministicMap{
				"key": "value",
			},
			expected: `{"key":"value"}`,
		},
		{
			name: "MultipleKeysSorted",
			input: DeterministicMap{
				"zebra":  "animal",
				"apple":  "fruit",
				"banana": "fruit",
				"carrot": "vegetable",
			},
			expected: `{"apple":"fruit","banana":"fruit","carrot":"vegetable","zebra":"animal"}`,
		},
		{
			name: "SpecialCharacters",
			input: DeterministicMap{
				"key with spaces": "value with spaces",
				"key\"quotes\"":   "value\"quotes\"",
				"key\nnewline":    "value\nnewline",
			},
			expected: `{"key\nnewline":"value\nnewline","key with spaces":"value with spaces","key\"quotes\"":"value\"quotes\""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))

			// Test determinism - marshal multiple times
			for i := 0; i < 10; i++ {
				result2, err := json.Marshal(tt.input)
				require.NoError(t, err)
				assert.Equal(t, string(result), string(result2), "marshaling should be deterministic")
			}
		})
	}
}

func TestDeterministicMap_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected DeterministicMap
		wantErr  bool
	}{
		{
			name:     "EmptyObject",
			input:    `{}`,
			expected: DeterministicMap{},
		},
		{
			name:     "Null",
			input:    `null`,
			expected: nil,
		},
		{
			name:  "SimpleObject",
			input: `{"key1": "value1", "key2": "value2"}`,
			expected: DeterministicMap{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:    "InvalidJson",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result DeterministicMap
			err := json.Unmarshal([]byte(tt.input), &result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeterministicMap_Merge(t *testing.T) {
	base := DeterministicMap{
		"key1": "value1",
		"key2": "value2",
	}

	other := DeterministicMap{
		"key2": "overridden",
		"key3": "value3",
	}

	result := base.Merge(other)

	expected := DeterministicMap{
		"key1": "value1",
		"key2": "overridden",
		"key3": "value3",
	}

	assert.Equal(t, expected, result)
	// Ensure original is not modified
	assert.Equal(t, "value2", base["key2"])
}

func TestDeterministicMap_String(t *testing.T) {
	m := DeterministicMap{
		"b": "2",
		"a": "1",
		"c": "3",
	}

	// Should be sorted
	expected := `{"a":"1","b":"2","c":"3"}`
	assert.Equal(t, expected, m.String())
}

func TestDeterministicMap_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    DeterministicMap
		expected string
	}{
		{
			name: "UnicodeCharacters",
			input: DeterministicMap{
				"你好":    "世界",
				"مرحبا": "عالم",
				"emoji": "🌍🚀",
				"mixed": "Hello世界🌍",
			},
			expected: `{"emoji":"🌍🚀","mixed":"Hello世界🌍","مرحبا":"عالم","你好":"世界"}`,
		},
		{
			name: "EmptyStringKeysAndValues",
			input: DeterministicMap{
				"":      "empty key",
				"empty": "",
				"both":  "",
			},
			expected: `{"":"empty key","both":"","empty":""}`,
		},
		{
			name: "NumericStringKeysSortedLexicographically",
			input: DeterministicMap{
				"1":   "one",
				"10":  "ten",
				"2":   "two",
				"20":  "twenty",
				"100": "hundred",
			},
			expected: `{"1":"one","10":"ten","100":"hundred","2":"two","20":"twenty"}`,
		},
		{
			name: "AllJSONSpecialCharacters",
			input: DeterministicMap{
				"tab":       "value\twith\ttab",
				"newline":   "value\nwith\nnewline",
				"return":    "value\rwith\rreturn",
				"backslash": "value\\with\\backslash",
				"quote":     "value\"with\"quote",
				"unicode":   "value\u0000with\u0001unicode",
			},
			expected: `{"backslash":"value\\with\\backslash","newline":"value\nwith\nnewline","quote":"value\"with\"quote","return":"value\rwith\rreturn","tab":"value\twith\ttab","unicode":"value\u0000with\u0001unicode"}`,
		},
		{
			name: "VeryLongValues",
			input: DeterministicMap{
				"long": strings.Repeat("a", 10000),
			},
			expected: fmt.Sprintf(`{"long":"%s"}`, strings.Repeat("a", 10000)),
		},
		{
			name: "CaseSensitiveKeys",
			input: DeterministicMap{
				"Key": "uppercase",
				"key": "lowercase",
				"KEY": "allcaps",
				"KeY": "mixed",
			},
			expected: `{"KEY":"allcaps","KeY":"mixed","Key":"uppercase","key":"lowercase"}`,
		},
		{
			name: "BooleanAndNullLikeStrings",
			input: DeterministicMap{
				"bool_true":  "true",
				"bool_false": "false",
				"null_str":   "null",
				"number":     "123.456",
			},
			expected: `{"bool_false":"false","bool_true":"true","null_str":"null","number":"123.456"}`,
		},
		{
			name: "KeysWithSpecialSortingCharacters",
			input: DeterministicMap{
				"a-b": "dash",
				"a_b": "underscore",
				"a.b": "dot",
				"a b": "space",
				"a:b": "colon",
				"a;b": "semicolon",
			},
			expected: `{"a b":"space","a-b":"dash","a.b":"dot","a:b":"colon","a;b":"semicolon","a_b":"underscore"}`,
		},
		{
			name: "JsonStringValues",
			input: DeterministicMap{
				"config":    `{"timeout": 30, "retries": 3}`,
				"array":     `["item1", "item2", "item3"]`,
				"nested":    `{"level1": {"level2": {"value": "deep"}}}`,
				"escaped":   `{"message": "Hello \"World\""}`,
				"multiline": `{"text": "line1\nline2\nline3"}`,
			},
			expected: `{"array":"[\"item1\", \"item2\", \"item3\"]","config":"{\"timeout\": 30, \"retries\": 3}","escaped":"{\"message\": \"Hello \\\"World\\\"\"}","multiline":"{\"text\": \"line1\\nline2\\nline3\"}","nested":"{\"level1\": {\"level2\": {\"value\": \"deep\"}}}"}`,
		},
		{
			name: "ComplexJsonInJsonScenario",
			input: DeterministicMap{
				"pipeline_config": `{"stages": ["build", "test", "deploy"], "parallel": true}`,
				"env_vars":        `{"NODE_ENV": "production", "API_KEY": "secret-key-123"}`,
				"matrix":          `[{"os": "linux", "arch": "amd64"}, {"os": "darwin", "arch": "arm64"}]`,
				"metadata":        `{"created_at": "2024-01-01T00:00:00Z", "version": "1.2.3"}`,
			},
			expected: `{"env_vars":"{\"NODE_ENV\": \"production\", \"API_KEY\": \"secret-key-123\"}","matrix":"[{\"os\": \"linux\", \"arch\": \"amd64\"}, {\"os\": \"darwin\", \"arch\": \"arm64\"}]","metadata":"{\"created_at\": \"2024-01-01T00:00:00Z\", \"version\": \"1.2.3\"}","pipeline_config":"{\"stages\": [\"build\", \"test\", \"deploy\"], \"parallel\": true}"}`,
		},
		{
			name: "MalformedJsonStrings",
			input: DeterministicMap{
				"invalid_json":   `{"broken": "json`,
				"not_json":       `this is not json at all`,
				"partial_escape": `{"key": "value with \" quote}`,
				"mixed_content":  `some text {"json": "inside"} more text`,
			},
			expected: `{"invalid_json":"{\"broken\": \"json","mixed_content":"some text {\"json\": \"inside\"} more text","not_json":"this is not json at all","partial_escape":"{\"key\": \"value with \\\" quote}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestDeterministicMap_ConcurrentAccess(t *testing.T) {
	// Note: DeterministicMap is just a map[string]string and is NOT thread-safe
	// This test documents that concurrent access would cause a race condition
	// In production, synchronization should be handled at a higher level
	t.Log("DeterministicMap is not thread-safe and requires external synchronization")
}

func TestDeterministicMap_UnmarshalExistingMap(t *testing.T) {
	// Test unmarshaling into an existing map
	m := &DeterministicMap{
		"existing": "value",
	}

	err := json.Unmarshal([]byte(`{"new": "value", "existing": "overridden"}`), m)
	require.NoError(t, err)

	// The existing map should be completely replaced, not merged
	assert.Len(t, *m, 2)
	assert.Equal(t, "overridden", (*m)["existing"])
	assert.Equal(t, "value", (*m)["new"])
}

func TestDeterministicMap_MarshalUnmarshalRoundTrip(t *testing.T) {
	// Test that marshal->unmarshal preserves data exactly
	original := DeterministicMap{
		"unicode":    "Hello 世界 🌍",
		"empty":      "",
		"spaces":     "  multiple  spaces  ",
		"number_str": "123.456789",
		"bool_str":   "true",
		"null_str":   "null",
		"escaped":    "line1\nline2\ttab",
		"quotes":     `"quoted"`,
	}

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	var restored DeterministicMap
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original, restored)

	// Marshal again and verify it's identical
	data2, err := json.Marshal(restored)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(data2), "marshaling should be deterministic")
}

func TestDeterministicMap_Integration_ParallelItem(t *testing.T) {
	// Test how DeterministicMap works within ParallelItem
	item := ParallelItem{
		Value: "",
		Params: DeterministicMap{
			"REGION": "us-east-1",
			"ENV":    "production",
			"DEBUG":  "true",
		},
	}

	// Marshal the entire ParallelItem
	data, err := json.Marshal(item)
	require.NoError(t, err)

	// Should have deterministic params ordering
	expected := `{"params":{"DEBUG":"true","ENV":"production","REGION":"us-east-1"}}`
	assert.Equal(t, expected, string(data))

	// Unmarshal and verify
	var restored ParallelItem
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)
	assert.Equal(t, item.Params, restored.Params)
}

func TestDeterministicMap_RealWorldChildDAGParams(t *testing.T) {
	// Test real-world scenario where child DAGs receive complex JSON parameters
	tests := []struct {
		name     string
		params   DeterministicMap
		expected string
	}{
		{
			name: "DeploymentConfiguration",
			params: DeterministicMap{
				"DEPLOYMENT_CONFIG": `{"service": "api-gateway", "replicas": 3, "resources": {"cpu": "500m", "memory": "1Gi"}}`,
				"ENVIRONMENT":       "production",
				"VERSION":           "v1.2.3",
				"ROLLBACK_ENABLED":  "true",
			},
			expected: `{"DEPLOYMENT_CONFIG":"{\"service\": \"api-gateway\", \"replicas\": 3, \"resources\": {\"cpu\": \"500m\", \"memory\": \"1Gi\"}}","ENVIRONMENT":"production","ROLLBACK_ENABLED":"true","VERSION":"v1.2.3"}`,
		},
		{
			name: "DataProcessingPipeline",
			params: DeterministicMap{
				"INPUT_SCHEMA":  `{"fields": [{"name": "id", "type": "string"}, {"name": "timestamp", "type": "datetime"}]}`,
				"TRANSFORM_OPS": `[{"op": "filter", "field": "status", "value": "active"}, {"op": "aggregate", "by": "region"}]`,
				"OUTPUT_FORMAT": "parquet",
				"PARTITION_BY":  `["year", "month", "day"]`,
			},
			expected: `{"INPUT_SCHEMA":"{\"fields\": [{\"name\": \"id\", \"type\": \"string\"}, {\"name\": \"timestamp\", \"type\": \"datetime\"}]}","OUTPUT_FORMAT":"parquet","PARTITION_BY":"[\"year\", \"month\", \"day\"]","TRANSFORM_OPS":"[{\"op\": \"filter\", \"field\": \"status\", \"value\": \"active\"}, {\"op\": \"aggregate\", \"by\": \"region\"}]"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ParallelItem with complex params
			item := ParallelItem{
				Params: tt.params,
			}

			// Marshal and verify deterministic output
			data, err := json.Marshal(item.Params)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))

			// Verify multiple marshals produce same result
			for i := 0; i < 5; i++ {
				data2, err := json.Marshal(item.Params)
				require.NoError(t, err)
				assert.Equal(t, string(data), string(data2), "marshal should be deterministic")
			}

			// Verify hash stability (simulating child DAG ID generation)
			hash1 := fmt.Sprintf("%x", data)
			data3, _ := json.Marshal(item.Params)
			hash2 := fmt.Sprintf("%x", data3)
			assert.Equal(t, hash1, hash2, "hash should be stable")
		})
	}
}
