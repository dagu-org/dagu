package stringutil_test

import (
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/stringutil"
)

func TestKeyValue(t *testing.T) {
	t.Run("NewKeyValue", func(t *testing.T) {
		kv := stringutil.NewKeyValue("foo", "bar")
		if kv.String() != "foo=bar" {
			t.Errorf("NewKeyValue() = %v, want %v", kv.String(), "foo=bar")
		}
	})

	tests := []struct {
		name      string
		input     stringutil.KeyValue
		wantKey   string
		wantValue string
		wantBool  bool
	}{
		{
			name:      "normal key-value",
			input:     stringutil.KeyValue("foo=bar"),
			wantKey:   "foo",
			wantValue: "bar",
			wantBool:  false,
		},
		{
			name:      "empty value",
			input:     stringutil.KeyValue("foo="),
			wantKey:   "foo",
			wantValue: "",
			wantBool:  false,
		},
		{
			name:      "empty key",
			input:     stringutil.KeyValue("=bar"),
			wantKey:   "",
			wantValue: "bar",
			wantBool:  false,
		},
		{
			name:      "no equals sign",
			input:     stringutil.KeyValue("foobar"),
			wantKey:   "foobar",
			wantValue: "",
			wantBool:  false,
		},
		{
			name:      "empty string",
			input:     stringutil.KeyValue(""),
			wantKey:   "",
			wantValue: "",
			wantBool:  false,
		},
		{
			name:      "bool true",
			input:     stringutil.KeyValue("flag=true"),
			wantKey:   "flag",
			wantValue: "true",
			wantBool:  true,
		},
		{
			name:      "bool false",
			input:     stringutil.KeyValue("flag=false"),
			wantKey:   "flag",
			wantValue: "false",
			wantBool:  false,
		},
		{
			name:      "bool invalid",
			input:     stringutil.KeyValue("flag=notbool"),
			wantKey:   "flag",
			wantValue: "notbool",
			wantBool:  false,
		},
		{
			name:      "multiple equals signs",
			input:     stringutil.KeyValue("foo=bar=baz"),
			wantKey:   "foo",
			wantValue: "bar=baz",
			wantBool:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.Key(); got != tt.wantKey {
				t.Errorf("KeyValue.Key() = %v, want %v", got, tt.wantKey)
			}
			if got := tt.input.Value(); got != tt.wantValue {
				t.Errorf("KeyValue.Value() = %v, want %v", got, tt.wantValue)
			}
			if got := tt.input.Bool(); got != tt.wantBool {
				t.Errorf("KeyValue.Bool() = %v, want %v", got, tt.wantBool)
			}
			if got := tt.input.String(); got != string(tt.input) {
				t.Errorf("KeyValue.String() = %v, want %v", got, string(tt.input))
			}
		})
	}
}

func TestKeyValueJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   stringutil.KeyValue
		want    string
		wantErr bool
	}{
		{
			name:    "normal key-value",
			input:   stringutil.KeyValue("foo=bar"),
			want:    `"foo=bar"`,
			wantErr: false,
		},
		{
			name:    "empty value",
			input:   stringutil.KeyValue("foo="),
			want:    `"foo="`,
			wantErr: false,
		},
		{
			name:    "empty key",
			input:   stringutil.KeyValue("=bar"),
			want:    `"=bar"`,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   stringutil.KeyValue(""),
			want:    `""`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test MarshalJSON
			got, err := json.Marshal(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyValue.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("KeyValue.MarshalJSON() = %v, want %v", string(got), tt.want)
			}

			// Test UnmarshalJSON
			var kv stringutil.KeyValue
			err = json.Unmarshal(got, &kv)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyValue.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if kv != tt.input {
				t.Errorf("KeyValue.UnmarshalJSON() = %v, want %v", kv, tt.input)
			}
		})
	}

	// Test UnmarshalJSON with invalid JSON
	t.Run("invalid json", func(t *testing.T) {
		var kv stringutil.KeyValue
		err := json.Unmarshal([]byte(`{invalid`), &kv)
		if err == nil {
			t.Error("KeyValue.UnmarshalJSON() expected error for invalid JSON")
		}
	})
}
