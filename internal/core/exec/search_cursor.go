// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import "errors"

var (
	ErrInvalidCursor = errors.New("invalid cursor")
)

// CursorResult contains a bounded result window and continuation state.
type CursorResult[T any] struct {
	Items      []T
	HasMore    bool
	NextCursor string
}
