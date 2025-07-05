package worker

import "sync"

type Worker struct {
	labels   sync.Map
	capacity int
	lock     sync.Mutex
}
