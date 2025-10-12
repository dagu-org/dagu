package core

import (
	"encoding/json"
	"sort"
)

// DeterministicMap is a map that marshals to JSON with sorted keys
// to ensure deterministic output for hashing and comparison.
type DeterministicMap map[string]string

// MarshalJSON implements json.Marshaler interface with deterministic output.
// Keys are always sorted alphabetically to ensure consistent JSON representation.
func (m DeterministicMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}

	// Get all keys and sort them
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the JSON manually to ensure order
	result := "{"
	for i, k := range keys {
		if i > 0 {
			result += ","
		}
		// Marshal key and value
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		valueJSON, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		result += string(keyJSON) + ":" + string(valueJSON)
	}
	result += "}"

	return []byte(result), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (m *DeterministicMap) UnmarshalJSON(data []byte) error {
	// First unmarshal into a regular map
	var temp map[string]string
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	*m = DeterministicMap(temp)
	return nil
}

// String returns a deterministic string representation of the map.
// This is useful for debugging and logging.
func (m DeterministicMap) String() string {
	if len(m) == 0 {
		return "{}"
	}

	data, err := m.MarshalJSON()
	if err != nil {
		return "{error: " + err.Error() + "}"
	}
	return string(data)
}

// Clone creates a deep copy of the map.
func (m DeterministicMap) Clone() DeterministicMap {
	if m == nil {
		return nil
	}

	clone := make(DeterministicMap, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// Merge combines this map with another map.
// Values from the other map will override values in this map for matching keys.
func (m DeterministicMap) Merge(other DeterministicMap) DeterministicMap {
	result := m.Clone()
	if result == nil {
		result = make(DeterministicMap)
	}

	for k, v := range other {
		result[k] = v
	}
	return result
}
