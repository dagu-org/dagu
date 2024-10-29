package model

import (
	"encoding/json"
	// "sync"
)

type Stats struct {
	Name string
}

func New(jobid string) *Stats {
	return &Stats{
		Name: jobid,
	}
}

func (s *Stats) StatsToJSON(str string) (*Stats, error) {
	stats := &Stats{}
	err := json.Unmarshal([]byte(str), stats)
	if err != nil {
		return nil, err
	}
	return stats, err
}

func (s *Stats) JSONToStats() ([]byte, error) {
	js, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
