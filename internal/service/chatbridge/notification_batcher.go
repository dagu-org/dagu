// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"sort"
	"sync"
	"time"
)

type notificationBucket struct {
	id          uint64
	destination string
	class       NotificationClass
	windowStart time.Time
	events      map[string]NotificationEvent
	timer       *time.Timer
}

// NotificationBatcher buffers notification subjects per destination and flush window.
type NotificationBatcher struct {
	mu            sync.Mutex
	urgentWindow  time.Duration
	successWindow time.Duration
	nextBucketID  uint64
	stopped       bool
	buckets       map[string]*notificationBucket
	runIndex      map[string]string
	ready         []NotificationPendingBatch
	readyCh       chan struct{}
}

// NewNotificationBatcher creates a new notification batcher.
func NewNotificationBatcher(urgentWindow, successWindow time.Duration) *NotificationBatcher {
	if urgentWindow <= 0 {
		urgentWindow = DefaultUrgentNotificationWindow
	}
	if successWindow <= 0 {
		successWindow = DefaultSuccessNotificationWindow
	}
	return &NotificationBatcher{
		urgentWindow:  urgentWindow,
		successWindow: successWindow,
		buckets:       make(map[string]*notificationBucket),
		runIndex:      make(map[string]string),
		readyCh:       make(chan struct{}, 1),
	}
}

// Stop prevents future flushes and stops all pending timers.
func (b *NotificationBatcher) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true
	timers := make([]*time.Timer, 0, len(b.buckets))
	for _, bucket := range b.buckets {
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
	}
	b.buckets = make(map[string]*notificationBucket)
	b.runIndex = make(map[string]string)
	b.ready = nil
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}
}

// DrainAndStop prevents future flushes, stops all pending timers, and returns
// the currently buffered batches for synchronous shutdown delivery.
func (b *NotificationBatcher) DrainAndStop() []NotificationPendingBatch {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return nil
	}

	b.stopped = true
	now := time.Now()
	timers := make([]*time.Timer, 0, len(b.buckets))
	drained := make([]NotificationPendingBatch, 0, len(b.ready)+len(b.buckets))
	drained = append(drained, append([]NotificationPendingBatch(nil), b.ready...)...)
	for _, bucket := range b.buckets {
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
		batch := notificationBatchFromBucket(bucket, now)
		if len(batch.Events) == 0 {
			continue
		}
		drained = append(drained, NotificationPendingBatch{
			Destination: bucket.destination,
			Batch:       batch,
		})
	}
	b.buckets = make(map[string]*notificationBucket)
	b.runIndex = make(map[string]string)
	b.ready = nil
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}

	sort.Slice(drained, func(i, j int) bool {
		if drained[i].Batch.Class != drained[j].Batch.Class {
			return drained[i].Batch.Class == NotificationClassUrgent
		}
		if !drained[i].Batch.WindowStart.Equal(drained[j].Batch.WindowStart) {
			return drained[i].Batch.WindowStart.Before(drained[j].Batch.WindowStart)
		}
		return drained[i].Destination < drained[j].Destination
	})

	return drained
}

// Enqueue adds a notification subject into the appropriate destination/window bucket.
func (b *NotificationBatcher) Enqueue(destination string, event NotificationEvent) bool {
	if destination == "" || event.Key == "" {
		return false
	}

	class, ok := NotificationClassForEvent(event)
	if !ok {
		return false
	}

	observedAt := event.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	snapshot := cloneNotificationEvent(event)
	snapshot.ObservedAt = observedAt
	groupKey := NotificationGroupKey(snapshot)
	if groupKey == "" {
		return false
	}
	destRunKey := notificationDestinationRunKey(destination, groupKey)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return false
	}

	if existingBucketKey, ok := b.runIndex[destRunKey]; ok {
		if existingBucket := b.buckets[existingBucketKey]; existingBucket != nil {
			if existingEvent, exists := existingBucket.events[groupKey]; exists {
				if existingEvent.Key == snapshot.Key {
					return true
				}
				delete(existingBucket.events, groupKey)
				delete(b.runIndex, destRunKey)
				if len(existingBucket.events) == 0 {
					if existingBucket.timer != nil {
						existingBucket.timer.Stop()
					}
					delete(b.buckets, existingBucketKey)
				}
			}
		}
	}
	if replaceReadyEvent(b.ready, destination, groupKey, snapshot) {
		return true
	}

	bucketKey := notificationBucketKey(destination, class)
	bucket, ok := b.buckets[bucketKey]
	if !ok {
		b.nextBucketID++
		bucket = &notificationBucket{
			id:          b.nextBucketID,
			destination: destination,
			class:       class,
			windowStart: observedAt,
			events:      make(map[string]NotificationEvent),
		}
		b.buckets[bucketKey] = bucket
		window := b.windowForClass(class)
		bucketID := bucket.id
		bucket.timer = time.AfterFunc(window, func() {
			b.readyBucket(bucketKey, bucketID)
		})
	}

	bucket.events[groupKey] = snapshot
	b.runIndex[destRunKey] = bucketKey
	return true
}

// ReadyC is signaled when one or more batches are ready for delivery.
func (b *NotificationBatcher) ReadyC() <-chan struct{} {
	return b.readyCh
}

// TakeReady returns all batches currently ready for delivery.
func (b *NotificationBatcher) TakeReady() []NotificationPendingBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.ready) == 0 {
		return nil
	}
	ready := append([]NotificationPendingBatch(nil), b.ready...)
	b.ready = nil
	return ready
}

// DiscardDestinations removes buffered and ready batches for destinations that
// are no longer configured.
func (b *NotificationBatcher) DiscardDestinations(destinations []string) {
	if len(destinations) == 0 {
		return
	}

	blocked := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		if destination == "" {
			continue
		}
		blocked[destination] = struct{}{}
	}
	if len(blocked) == 0 {
		return
	}

	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	timers := make([]*time.Timer, 0)
	for bucketKey, bucket := range b.buckets {
		if bucket == nil {
			continue
		}
		if _, ok := blocked[bucket.destination]; !ok {
			continue
		}
		if bucket.timer != nil {
			timers = append(timers, bucket.timer)
		}
		delete(b.buckets, bucketKey)
		for runKey := range bucket.events {
			delete(b.runIndex, notificationDestinationRunKey(bucket.destination, runKey))
		}
	}

	if len(b.ready) > 0 {
		filtered := make([]NotificationPendingBatch, 0, len(b.ready))
		for _, pending := range b.ready {
			if _, ok := blocked[pending.Destination]; ok {
				continue
			}
			filtered = append(filtered, pending)
		}
		b.ready = filtered
	}
	b.mu.Unlock()

	for _, timer := range timers {
		timer.Stop()
	}
}

func (b *NotificationBatcher) readyBucket(bucketKey string, bucketID uint64) {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}

	bucket := b.buckets[bucketKey]
	if bucket == nil || bucket.id != bucketID {
		b.mu.Unlock()
		return
	}

	delete(b.buckets, bucketKey)
	for runKey := range bucket.events {
		delete(b.runIndex, notificationDestinationRunKey(bucket.destination, runKey))
	}
	batch := notificationBatchFromBucket(bucket, time.Now())
	if len(batch.Events) > 0 {
		b.ready = append(b.ready, NotificationPendingBatch{
			Destination: bucket.destination,
			Batch:       batch,
		})
		select {
		case b.readyCh <- struct{}{}:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *NotificationBatcher) windowForClass(class NotificationClass) time.Duration {
	if class == NotificationClassUrgent {
		return b.urgentWindow
	}
	return b.successWindow
}

func replaceReadyEvent(ready []NotificationPendingBatch, destination, groupKey string, snapshot NotificationEvent) bool {
	for batchIndex := range ready {
		if ready[batchIndex].Destination != destination {
			continue
		}
		for eventIndex := range ready[batchIndex].Batch.Events {
			existing := ready[batchIndex].Batch.Events[eventIndex]
			if NotificationGroupKey(existing) != groupKey {
				continue
			}
			if existing.Key == snapshot.Key {
				return true
			}
			ready[batchIndex].Batch.Events[eventIndex] = snapshot
			return true
		}
	}
	return false
}
