package model

import (
	"sync"
)

type Stats struct {
	RunningDags int `json:"running_dags"`
	mutex       sync.Mutex
}

func New() *Stats {
	return &Stats{}
}

func (s *Stats) IncrementRunningDags() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.RunningDags++
}

func (s *Stats) DecrementRunningDags() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.RunningDags--
}

func (s *Stats) GetRunningDags() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.RunningDags
}
