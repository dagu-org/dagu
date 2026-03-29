// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import "context"

// Store persists and queries event-feed entries.
type Store interface {
	Append(ctx context.Context, entry *Entry) error
	Query(ctx context.Context, filter QueryFilter) (*QueryResult, error)
	Close() error
}

// Recorder records event-feed entries.
type Recorder interface {
	Record(ctx context.Context, entry Entry) error
}
