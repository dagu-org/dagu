package model

import (
	"encoding/json"
	// "fmt"
	// "strings"
	"github.com/dagu-org/dagu/internal/dag"
)

// queue is an interface to read and append jobid's into queue.

type Queue struct {
	Name   string   `json:"Name"`
	Params []string `json:"Params"`
}

type QueueFile struct {
	File  string
	Queue *Queue
}

func (que *Queue) QueueFromJson(s string) (*Queue, error) {
	queue := &Queue{}
	err := json.Unmarshal([]byte(s), queue)
	if err != nil {
		return nil, err
	}
	return queue, err
}

func NewQueue(d *dag.DAG) *Queue {
	return &Queue{
		Name:   d.Name,
		Params: d.Params,
	}
}

func (que *Queue) QueueToJson() ([]byte, error) {
	js, err := json.Marshal(que)
	if err != nil {
		return []byte{}, err
	}
	return js, nil
}
