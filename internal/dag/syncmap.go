// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
