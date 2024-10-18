package stats

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

type StatsStore struct {
	stats *model.Stats
	mutex sync.RWMutex
	dir   string
}

func NewStatsStore(dirPath string) *StatsStore {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil
	}

	return &StatsStore{
		stats: &model.Stats{},
		dir:   dirPath,
	}
}

func (s *StatsStore) Create() error {
	filePath := filepath.Join(s.dir, "stats.json")
	_, err := util.OpenOrCreateFile(filePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *StatsStore) IncrementRunningDags() error {
	s.mutex.Lock()
	//defer s.mutex.Unlock()
	_ = s.loadFromFile()
	log.Print("incrementing   ", s.dir, s.stats)
	s.stats.RunningDags++
	write_stat := s.writeToFile()
	s.mutex.Unlock()
	return write_stat
}

func (s *StatsStore) DecrementRunningDags() error {
	log.Print("decrementing")
	_ = s.loadFromFile()
	//Lock Mutex
	s.mutex.Lock()
	if s.stats.RunningDags > 0 {
		s.stats.RunningDags--
	}
	write_stat := s.writeToFile()
	//Unlock Mutex
	//return s.writeToFile()
	defer s.mutex.Unlock()
	return write_stat
}

func (s *StatsStore) GetRunningDags() (int, error) {
	s.mutex.Lock()
	//defer s.mutex.Unlock()
	_ = s.loadFromFile()
	s.mutex.Unlock()
	return s.stats.RunningDags, nil
}

func (s *StatsStore) writeToFile() error {
	filePath := filepath.Join(s.dir, "stats.json")
	data, err := json.Marshal(s.stats)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0600)
}

func (s *StatsStore) loadFromFile() error {
	filePath := filepath.Join(s.dir, "stats.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, which is fine
		}
		return err
	}

	return json.Unmarshal(data, s.stats)
}
