// Copyright (C) 2024 The Dagu Authors
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

package stats

import (
	"encoding/json"
	"sync"

	// "fmt"
	"log"
	"os"

	// "strings"
	// "errors"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

type StatsStore struct {
	dir   string
	mutex sync.RWMutex
	Stats []*model.Stats `json:"stats"`
}

func NewStatsStore(dirPath string) *StatsStore {
	_ = os.MkdirAll(dirPath, 0755)
	return &StatsStore{dir: dirPath}
}

func (store *StatsStore) Create() error {
	statsPath := filepath.Join(store.dir, "stats.json")
	_, err := util.OpenOrCreateFile(statsPath)
	if err != nil {
		return err
	}
	return nil
}

func (store *StatsStore) Save() error {
	statsPath := filepath.Join(store.dir, "stats.json")
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	return os.WriteFile(statsPath, data, 0600)
}

func (store *StatsStore) Load() error {
	statsPath := filepath.Join(store.dir, "stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			store.Stats = []*model.Stats{}
			return nil
		}
		return err
	}
	return json.Unmarshal(data, store)
}

func (store *StatsStore) IncrementRunningDags(jobid string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	err := store.Load()
	if err != nil {
		return err
	}
	// log.Print("data:", data)
	log.Print("incrementing: ", jobid)
	store.Stats = append(store.Stats, &model.Stats{Name: jobid})
	return store.Save()
}

func (store *StatsStore) DecrementRunningDags(jobid string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	err := store.Load()
	if err != nil {
		return err
	}
	for i := 0; i < len(store.Stats); i++ {
		if store.Stats[i].Name == jobid {
			store.Stats = append(store.Stats[:i], store.Stats[i+1:]...) // Remove the item
			err := store.Save()
			if err != nil {
				return err
			} else {
				return nil // Item found and deleted
			}
		}
	}
	log.Print("decrementing: ", jobid)
	err = store.Save()
	return err
}

func (store *StatsStore) GetRunningDags() (int, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	err := store.Load()
	if err != nil {
		return 0, err
	}
	lenQ := len(store.Stats)
	return lenQ, nil
}
