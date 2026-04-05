// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMockQueueStoreListCursorNilResult(t *testing.T) {
	t.Parallel()

	store := &MockQueueStore{}
	store.On("ListCursor", mock.Anything, "queue", "", 10).Return(nil, assert.AnError)

	result, err := store.ListCursor(context.Background(), "queue", "", 10)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, result.Items)
	assert.False(t, result.HasMore)
}
