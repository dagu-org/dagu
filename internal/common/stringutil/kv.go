package stringutil

import (
	"encoding/json"
	"strconv"
	"strings"
)

type KeyValue string

func NewKeyValue(key, value string) KeyValue {
	return KeyValue(key + "=" + value)
}

func (kv KeyValue) Key() string {
	key, _, _ := strings.Cut(string(kv), "=")
	return key
}

func (kv KeyValue) Value() string {
	_, value, found := strings.Cut(string(kv), "=")
	if !found {
		return ""
	}
	return value
}

func (kv KeyValue) Bool() bool {
	v, err := strconv.ParseBool(kv.Value())
	if err != nil {
		return false
	}
	return v
}

func (kv KeyValue) String() string {
	return string(kv)
}

func (kv KeyValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(kv.String())
}

func (kv *KeyValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*kv = KeyValue(s)
	return nil
}

// KeyValuesToMap converts a slice of "KEY=VALUE" strings to a map.
// Only entries with valid "KEY=VALUE" format (containing exactly one '=' with non-empty key) are included.
// Values can be empty (e.g., "KEY=" results in map["KEY"] = "").
// Entries without '=' are skipped.
func KeyValuesToMap(kvSlice []string) map[string]string {
	result := make(map[string]string, len(kvSlice))
	for _, kv := range kvSlice {
		key, value, found := strings.Cut(kv, "=")
		if found {
			result[key] = value
		}
	}
	return result
}
