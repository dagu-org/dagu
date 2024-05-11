package filecache

import (
	"fmt"
	"os"
	"sync"
)

type Cache[T any] struct {
	entries sync.Map
}

type Entry[T any] struct {
	Data         T
	LastModified int64
}

func New[T any]() *Cache[T] {
	return &Cache[T]{}
}

func (c *Cache[T]) Store(fileName string, data T, lastModified int64) {
	c.entries.Store(fileName, Entry[T]{Data: data, LastModified: lastModified})
}

func (c *Cache[T]) Invalidate(fileName string) {
	c.entries.Delete(fileName)
}

func (c *Cache[T]) LoadLatest(fileName string, loader func() (T, error)) (T, error) {
	stale, lastModified, err := c.IsStale(fileName, c.LastModified(fileName))
	if err != nil {
		var zero T
		return zero, err
	}
	if stale {
		data, err := loader()
		if err != nil {
			var zero T
			return zero, err
		}
		c.Store(fileName, data, lastModified)
		return data, nil
	}
	item, _ := c.entries.Load(fileName)
	entry := item.(Entry[T])
	return entry.Data, nil
}

func (c *Cache[T]) LastModified(fileName string) int64 {
	item, ok := c.entries.Load(fileName)
	if !ok {
		return 0
	}
	entry := item.(Entry[T])
	return entry.LastModified
}

func (c *Cache[T]) Load(fileName string) (T, bool) {
	item, ok := c.entries.Load(fileName)
	if !ok {
		var zero T
		return zero, false
	}
	entry := item.(Entry[T])
	return entry.Data, true
}

func (c *Cache[T]) IsStale(fileName string, lastModified int64) (bool, int64, error) {
	fi, err := os.Stat(fileName)
	if err != nil {
		return true, 0, fmt.Errorf("failed to stat file %s: %w", fileName, err)
	}
	t := fi.ModTime().Unix()
	return lastModified < t, t, nil
}
