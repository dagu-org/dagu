package digraph

import (
	"encoding/json"
	"strings"
	"sync"
)

// SyncMap wraps a sync.Map to make it JSON serializable.
type SyncMap struct {
	sync.Map
	variables map[string]string
	dirty     bool
}

func (m *SyncMap) Store(key, value any) {
	m.Map.Store(key, value)
	m.dirty = true
}

// Variables returns the map of variables.
// A variable is a string in the form of "key=value".
func (m *SyncMap) Variables() map[string]string {
	if !m.dirty && m.variables != nil {
		return m.variables
	}
	vars := make(map[string]string)
	m.Range(func(_, value any) bool {
		parts := strings.SplitN(value.(string), "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
		return true
	})
	m.variables = vars
	m.dirty = false
	return vars
}

func (m *SyncMap) MarshalJSON() ([]byte, error) {
	tmpMap := make(map[string]any)

	m.Range(func(k, v any) bool {
		tmpMap[k.(string)] = v
		return true
	})

	return json.Marshal(tmpMap)
}

func (m *SyncMap) UnmarshalJSON(data []byte) error {
	var tmpMap map[string]any
	if err := json.Unmarshal(data, &tmpMap); err != nil {
		return err
	}

	for key, value := range tmpMap {
		m.Store(key, value)
	}

	return nil
}

func (m *SyncMap) MarshalJSONIndent(prefix, indent string) ([]byte, error) {
	tmpMap := make(map[string]any)

	m.Range(func(k, v any) bool {
		tmpMap[k.(string)] = v
		return true
	})

	return json.MarshalIndent(tmpMap, prefix, indent)
}
