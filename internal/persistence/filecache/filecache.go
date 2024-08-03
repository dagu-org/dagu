// Copyright (C) 2024 The Daguflow/Dagu Authors
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

package filecache

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/rand"
)

type Entry[T any] struct {
	Data         T
	Size         int64
	LastModified int64
	ExpiresAt    time.Time
}

// TODO: Consider replacing this with golang-lru:
// https://github.com/hashicorp/golang-lru
type Cache[T any] struct {
	entries  sync.Map
	capacity int
	ttl      time.Duration
	items    atomic.Int32
	stopCh   chan struct{}
}

func New[T any](cap int, ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		capacity: cap,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
}

func (c *Cache[T]) Stop() {
	close(c.stopCh)
}

func (c *Cache[T]) StartEviction() {
	go func() {
		timer := time.NewTimer(time.Minute)
		for {
			select {
			case <-timer.C:
				timer.Reset(time.Minute)
				c.evict()
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *Cache[T]) evict() {
	c.entries.Range(func(key, value any) bool {
		entry := value.(Entry[T])
		if time.Now().After(entry.ExpiresAt) {
			c.entries.Delete(key)
			c.items.Add(-1)
		}
		return true
	})
	if c.capacity > 0 && int(c.items.Load()) > c.capacity {
		c.entries.Range(func(key, _ any) bool {
			c.items.Add(-1)
			c.entries.Delete(key)
			return int(c.items.Load()) > c.capacity
		})
	}
}

func (c *Cache[T]) StopEviction() {
	c.stopCh <- struct{}{}
}

func (c *Cache[T]) Store(fileName string, data T, fi os.FileInfo) {
	c.items.Add(1)
	c.entries.Store(
		fileName, newEntry(data, fi.Size(), fi.ModTime().Unix(), c.ttl))
}

func (c *Cache[T]) Invalidate(fileName string) {
	c.items.Add(-1)
	c.entries.Delete(fileName)
}

func (c *Cache[T]) LoadLatest(
	fileName string, loader func() (T, error),
) (T, error) {
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

func (*Cache[T]) IsStale(
	fileName string, entry Entry[T],
) (bool, os.FileInfo, error) {
	fi, err := os.Stat(fileName)
	if err != nil {
		return true, fi, fmt.Errorf("failed to stat file %s: %w", fileName, err)
	}
	t := fi.ModTime().Unix()
	return entry.LastModified < t || entry.Size != fi.Size(), fi, nil
}

func newEntry[T any](
	data T, size int64, lastModified int64, ttl time.Duration,
) Entry[T] {
	expiresAt := time.Now().Add(ttl)
	// Add random jitter to avoid thundering herd
	randMin := time.Duration(rand.Intn(60)) * time.Minute
	expiresAt = expiresAt.Add(randMin)

	return Entry[T]{
		Data:         data,
		Size:         size,
		LastModified: lastModified,
		ExpiresAt:    expiresAt,
	}
}
