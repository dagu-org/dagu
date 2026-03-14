// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetSSEHeaders(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()

	SetSSEHeaders(w)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

func TestSetLegacyStreamDeprecationHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	SetLegacyStreamDeprecationHeaders(w)

	assert.Equal(t, legacyStreamDeprecatedAt, w.Header().Get("Deprecation"))
	assert.Equal(t, legacyStreamSunset, w.Header().Get("Sunset"))
}
