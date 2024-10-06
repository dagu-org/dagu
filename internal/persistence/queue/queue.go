package queue

import (
	"encoding/json"
	"sync"

	// "fmt"
	"log"
	"os"

	// "strings"
	// "errors"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

// type QueueItem struct {
// 	DAGFile string   `json:"dag"`
// 	Params  []string `json:"params"`
// }

type jsonStore struct {
	dir       string
	queueLock sync.Mutex
	Dags      []*model.Queue `json:"dags"`
}

func NewQueueStore(dirPath string) *jsonStore {
	// dir := filepath.Join(dirPath , "queue.json")
	_ = os.MkdirAll(dirPath, 0755)
	return &jsonStore{dir: dirPath}
}

func (store *jsonStore) Create() error {
	// dir := filepath.Join(dirPath , "queue.json")
	queuePath := filepath.Join(store.dir, "queue.json")
	_, err := util.OpenOrCreateFile(queuePath)
	if err != nil {
		return err
	}
	return nil
}

func (store *jsonStore) Save() error {
	queuePath := filepath.Join(store.dir, "queue.json")
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	return os.WriteFile(queuePath, data, 0644)
}

// func (store *jsonStore) Open(queue persistence.QueueStore) error{
// 	// is this working?
// 	log.Print("json opened", queue)
// 	return nil
// }

func (store *jsonStore) Load() error {
	queuePath := filepath.Join(store.dir, "queue.json")
	data, err := os.ReadFile(queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			store.Dags = []*model.Queue{}
			return nil
		}
		return err
	}
	return json.Unmarshal(data, store)
}

func (store *jsonStore) QueueLength() int {
	store.queueLock.Lock()
	defer store.queueLock.Unlock()
	store.Load()
	lenQ := len(store.Dags)
	return lenQ
}

func (store *jsonStore) Enqueue(d *dag.DAG) error {
	store.queueLock.Lock()
	defer store.queueLock.Unlock()
	store.Load()
	// log.Print("data:", data)
	store.Dags = append(store.Dags, &model.Queue{Name: d.Location, Params: d.Params})
	return store.Save()
}

func (store *jsonStore) Dequeue() (*model.Queue, error) {
	store.queueLock.Lock()
	defer store.queueLock.Unlock()
	store.Load()
	if len(store.Dags) == 0 {
		return nil, nil
	}
	item := store.Dags[0]
	log.Print("dequeue", item)
	store.Dags = store.Dags[1:]
	err := store.Save()
	return item, err
}
