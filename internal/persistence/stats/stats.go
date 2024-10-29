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
	return os.WriteFile(statsPath, data, 0644)
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
	store.Load()
	// log.Print("data:", data)
	log.Print("incrementing: ", jobid)
	store.Stats = append(store.Stats, &model.Stats{Name: jobid})
	return store.Save()
}

func (store *StatsStore) DecrementRunningDags(jobid string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.Load()
	var item string
	for i := 0; i < len(store.Stats); i++ {
		if store.Stats[i].Name == jobid {
			item = store.Stats[i].Name
			store.Stats = append(store.Stats[:i], store.Stats[i+1:]...) // Remove the item
			err := store.Save()
			if err != nil {
				return err
			} else {
				return nil // Item found and deleted
			}
			log.Print("jobid", jobid)
		}
	}
	log.Print("decrementing: ", item)
	err := store.Save()
	return err
}

func (store *StatsStore) GetRunningDags() (int, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	store.Load()
	lenQ := len(store.Stats)
	return lenQ, nil
}
