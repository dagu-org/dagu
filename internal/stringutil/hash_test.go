package stringutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase58EncodeSHA256(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // We'll validate format rather than exact value
	}{
		{
			name:  "simple string",
			input: "hello",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "child DAG ID format",
			input: "12345:process-data:{\"REGION\":\"us-east-1\",\"VERSION\":\"1.0.0\"}",
		},
		{
			name:  "unicode characters",
			input: "Hello 世界 🌍",
		},
		{
			name:  "long input",
			input: "parent-dag-run-id-12345:parallel-step-name:{\"param1\":\"value1\",\"param2\":\"value2\",\"param3\":\"value3\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Base58EncodeSHA256(tt.input)
			
			// Validate result is non-empty
			assert.NotEmpty(t, result, "base58 encoded hash should not be empty")
			
			// Validate result contains only base58 characters
			for _, c := range result {
				assert.Contains(t, base58Alphabet, string(c), "result should only contain base58 characters")
			}
			
			// Validate consistency - same input should always produce same output
			result2 := Base58EncodeSHA256(tt.input)
			assert.Equal(t, result, result2, "same input should produce same output")
		})
	}
}

func TestBase58Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "single zero byte",
			input:    []byte{0},
			expected: "1",
		},
		{
			name:     "multiple zero bytes",
			input:    []byte{0, 0, 0},
			expected: "111",
		},
		{
			name:     "simple bytes",
			input:    []byte{1, 2, 3},
			expected: "Ldp",
		},
		{
			name:     "SHA256 hash",
			input:    func() []byte { h := sha256.Sum256([]byte("test")); return h[:] }(),
			expected: "Bjj4AWTNrjQVHqgWbP2XaxXz4DYH1WZMyERHxsad7b2w",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Base58Encode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBase58Decode(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []byte
		expectError bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []byte{},
		},
		{
			name:     "single 1",
			input:    "1",
			expected: []byte{0},
		},
		{
			name:     "multiple 1s",
			input:    "111",
			expected: []byte{0, 0, 0},
		},
		{
			name:     "simple base58",
			input:    "Ldp",
			expected: []byte{1, 2, 3},
		},
		{
			name:        "invalid character 0",
			input:       "1230",
			expectError: true,
		},
		{
			name:        "invalid character O",
			input:       "123O",
			expectError: true,
		},
		{
			name:        "invalid character l",
			input:       "123l",
			expectError: true,
		},
		{
			name:        "invalid character I",
			input:       "123I",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Base58Decode(tt.input)
			
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBase58RoundTrip(t *testing.T) {
	// Test that encode->decode produces original input
	testCases := [][]byte{
		{},
		{0},
		{0, 0, 0},
		{1, 2, 3, 4, 5},
		{255, 254, 253, 252, 251},
		func() []byte { h := sha256.Sum256([]byte("test data")); return h[:] }(),
	}

	for i, original := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			encoded := Base58Encode(original)
			decoded, err := Base58Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, original, decoded, "round trip should preserve original bytes")
		})
	}
}

func TestBase58EncodeSHA256_ChildDAGScenarios(t *testing.T) {
	// Test specific scenarios for child DAG ID generation
	tests := []struct {
		name             string
		parentRunID      string
		stepName         string
		params           string
		expectedLength   int // Expected length range
	}{
		{
			name:           "simple child DAG",
			parentRunID:    "parent-12345",
			stepName:       "process",
			params:         `{"env":"prod"}`,
			expectedLength: 40, // Base58 encoded SHA256 is typically 43-44 chars
		},
		{
			name:           "child DAG with complex params",
			parentRunID:    "workflow-abc-123",
			stepName:       "etl-pipeline",
			params:         `{"AWS_REGION":"us-east-1","BATCH_SIZE":"1000","MODE":"parallel"}`,
			expectedLength: 40,
		},
		{
			name:           "nested child DAG scenario",
			parentRunID:    "root-workflow:child-workflow:grandchild-12345",
			stepName:       "data-processor",
			params:         `{"input":"/data/raw","output":"/data/processed"}`,
			expectedLength: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct the input as it would be in real usage
			input := fmt.Sprintf("%s:%s:%s", tt.parentRunID, tt.stepName, tt.params)
			
			result := Base58EncodeSHA256(input)
			
			// Validate result properties
			assert.NotEmpty(t, result)
			assert.GreaterOrEqual(t, len(result), tt.expectedLength)
			assert.LessOrEqual(t, len(result), tt.expectedLength+4) // Allow some variance
			
			// Ensure different inputs produce different outputs
			altInput := input + "-modified"
			altResult := Base58EncodeSHA256(altInput)
			assert.NotEqual(t, result, altResult, "different inputs should produce different hashes")
			
			// Validate no ambiguous characters
			for _, c := range result {
				assert.NotContains(t, "0OlI", string(c), "result should not contain ambiguous characters")
			}
		})
	}
}

func TestBase58Error(t *testing.T) {
	// Test the error type
	err := &base58Error{char: '0'}
	assert.Equal(t, "invalid base58 character: 0", err.Error())
	
	err = &base58Error{char: 'O'}
	assert.Equal(t, "invalid base58 character: O", err.Error())
}

func BenchmarkBase58EncodeSHA256(b *testing.B) {
	input := "parent-dag-run-12345:step-name:{\"key\":\"value\"}"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Base58EncodeSHA256(input)
	}
}

func BenchmarkBase58Encode(b *testing.B) {
	h := sha256.Sum256([]byte("benchmark test data"))
	data := h[:]
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Base58Encode(data)
	}
}

// TestBase58Compatibility validates our implementation against known test vectors
func TestBase58Compatibility(t *testing.T) {
	// Test vectors from Bitcoin base58 implementation
	testVectors := []struct {
		hex    string
		base58 string
	}{
		{
			hex:    "",
			base58: "",
		},
		{
			hex:    "00",
			base58: "1",
		},
		{
			hex:    "0000",
			base58: "11",
		},
		{
			hex:    "00000000000000000000",
			base58: "1111111111",
		},
		{
			hex:    "61",
			base58: "2g",
		},
		{
			hex:    "626262",
			base58: "a3gV",
		},
		{
			hex:    "636363",
			base58: "aPEr",
		},
		{
			hex:    "516b6fcd0f",
			base58: "ABnLTmg",
		},
		{
			hex:    "bf4f89001e670274dd",
			base58: "3SEo3LWLoPntC",
		},
		{
			hex:    "572e4794",
			base58: "3EFU7m",
		},
		{
			hex:    "ecac89cad93923c02321",
			base58: "EJDM8drfXA6uyA",
		},
		{
			hex:    "10c8511e",
			base58: "Rt5zm",
		},
		{
			hex:    "00000000000000000000000000000000000000000000000000000000000000",
			base58: "1111111111111111111111111111111",
		},
	}

	for _, tv := range testVectors {
		t.Run(tv.hex, func(t *testing.T) {
			// Decode hex to bytes
			bytes, err := hex.DecodeString(tv.hex)
			require.NoError(t, err)
			
			// Test encoding
			encoded := Base58Encode(bytes)
			assert.Equal(t, tv.base58, encoded, "encoding should match test vector")
			
			// Test decoding
			decoded, err := Base58Decode(tv.base58)
			require.NoError(t, err)
			assert.Equal(t, bytes, decoded, "decoding should match original bytes")
		})
	}
}