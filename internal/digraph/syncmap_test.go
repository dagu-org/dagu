package digraph

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSyncMap_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		want    string
		wantErr bool
	}{
		{
			name:    "Empty map",
			input:   map[string]any{},
			want:    "{}",
			wantErr: false,
		},
		{
			name: "Map with string values",
			input: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			want:    `{"key1":"value1","key2":"value2"}`,
			wantErr: false,
		},
		{
			name: "Map with mixed value types",
			input: map[string]any{
				"string": "value",
				"number": 42,
				"bool":   true,
				"null":   nil,
			},
			want:    `{"bool":true,"null":null,"number":42,"string":"value"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &SyncMap{}
			for k, v := range tt.input {
				m.Store(k, v)
			}

			got, err := m.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncMap.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !jsonEqual(string(got), tt.want) {
				t.Errorf("SyncMap.MarshalJSON() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestSyncMap_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "Empty JSON object",
			input:   "{}",
			want:    map[string]any{},
			wantErr: false,
		},
		{
			name:  "JSON object with string values",
			input: `{"key1":"value1","key2":"value2"}`,
			want: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:  "JSON object with mixed value types",
			input: `{"string":"value","number":42,"bool":true,"null":null}`,
			want: map[string]any{
				"string": "value",
				"number": float64(42), // JSON numbers are unmarshaled as float64
				"bool":   true,
				"null":   nil,
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{"key":"value"`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &SyncMap{}
			err := m.UnmarshalJSON([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncMap.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				got := make(map[string]any)
				m.Range(func(k, v any) bool {
					got[k.(string)] = v
					return true
				})

				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("SyncMap.UnmarshalJSON() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSyncMap_RoundTrip(t *testing.T) {
	original := &SyncMap{}
	original.Store("string", "value")
	original.Store("number", float64(42))
	original.Store("bool", true)
	original.Store("null", nil)

	// Marshal
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	roundTripped := &SyncMap{}
	err = roundTripped.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Compare
	originalMap := make(map[string]any)
	roundTrippedMap := make(map[string]any)

	original.Range(func(k, v any) bool {
		originalMap[k.(string)] = v
		return true
	})

	roundTripped.Range(func(k, v any) bool {
		roundTrippedMap[k.(string)] = v
		return true
	})

	if !reflect.DeepEqual(originalMap, roundTrippedMap) {
		t.Errorf("Round-trip failed. Original: %v, Round-tripped: %v", originalMap, roundTrippedMap)
	}
}

// jsonEqual compares two JSON strings for equality, ignoring field order
func jsonEqual(a, b string) bool {
	var j1, j2 any
	if err := json.Unmarshal([]byte(a), &j1); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &j2); err != nil {
		return false
	}
	return reflect.DeepEqual(j1, j2)
}
