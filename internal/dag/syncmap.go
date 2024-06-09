package dag

import (
	"encoding/json"
	"sync"
)

// SyncMap wraps a sync.Map to make it JSON serializable.
type SyncMap struct{ sync.Map }

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
