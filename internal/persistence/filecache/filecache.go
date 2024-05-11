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
	Size         int64
	LastModified int64
}

func New[T any]() *Cache[T] {
	return &Cache[T]{}
}

func (c *Cache[T]) Store(fileName string, data T, fi os.FileInfo) {
	c.entries.Store(fileName, Entry[T]{Data: data, Size: fi.Size(), LastModified: fi.ModTime().Unix()})
}

func (c *Cache[T]) Invalidate(fileName string) {
	c.entries.Delete(fileName)
}

func (c *Cache[T]) LoadLatest(fileName string, loader func() (T, error)) (T, error) {
	stale, lastModified, err := c.IsStale(fileName, c.Entry(fileName))
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

func (c *Cache[T]) Entry(fileName string) Entry[T] {
	item, ok := c.entries.Load(fileName)
	if !ok {
		return Entry[T]{}
	}
	return item.(Entry[T])
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

func (c *Cache[T]) IsStale(fileName string, entry Entry[T]) (bool, os.FileInfo, error) {
	fi, err := os.Stat(fileName)
	if err != nil {
		return true, fi, fmt.Errorf("failed to stat file %s: %w", fileName, err)
	}
	t := fi.ModTime().Unix()
	return entry.LastModified < t || entry.Size != fi.Size(), fi, nil
}
