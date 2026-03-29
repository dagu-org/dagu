// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const DefaultWriteTimeout = 250 * time.Millisecond

// Service wraps an event-feed store with bounded write behavior.
type Service struct {
	store        Store
	writeTimeout time.Duration
}

// Option configures a Service.
type Option func(*Service)

// WithWriteTimeout overrides the bounded append timeout.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(s *Service) {
		s.writeTimeout = timeout
	}
}

// New constructs a Service.
func New(store Store, opts ...Option) *Service {
	svc := &Service{
		store:        store,
		writeTimeout: DefaultWriteTimeout,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Record appends an event-feed entry with a short bounded timeout.
func (s *Service) Record(ctx context.Context, entry Entry) error {
	if s == nil || s.store == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}

	recordCtx := context.WithoutCancel(ctx)
	if s.writeTimeout > 0 {
		var cancel context.CancelFunc
		recordCtx, cancel = context.WithTimeout(recordCtx, s.writeTimeout)
		defer cancel()
	}

	return s.store.Append(recordCtx, &entry)
}

// Query retrieves event-feed entries.
func (s *Service) Query(ctx context.Context, filter QueryFilter) (*QueryResult, error) {
	if s == nil || s.store == nil {
		return &QueryResult{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.Query(ctx, filter)
}

// Close releases store resources.
func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}
