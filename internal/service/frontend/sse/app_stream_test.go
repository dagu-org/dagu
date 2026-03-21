// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteAppEventFrame(t *testing.T) {
	recorder := httptest.NewRecorder()

	err := writeAppEventFrame(recorder, AppEvent{
		Type:     AppEventTypeDAGChanged,
		FileName: "example.yaml",
		Reason:   "updated",
	})
	if err != nil {
		t.Fatalf("writeAppEventFrame() error = %v", err)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "event: dag.changed\n") {
		t.Fatalf("expected SSE event frame, got %q", body)
	}
	if !strings.Contains(body, `"fileName":"example.yaml"`) {
		t.Fatalf("expected serialized file name in frame, got %q", body)
	}
	if !strings.Contains(body, `"reason":"updated"`) {
		t.Fatalf("expected serialized reason in frame, got %q", body)
	}
}

func TestRecursiveWatcherStopIsIdempotent(_ *testing.T) {
	watcher := &recursiveWatcher{
		done: make(chan struct{}),
	}

	watcher.Stop()
	watcher.Stop()
}

func TestAppStreamServiceShutdownIsIdempotent(_ *testing.T) {
	service := &AppStreamService{
		cancel: func() {},
		watchers: []*recursiveWatcher{
			{done: make(chan struct{})},
		},
	}

	service.Shutdown()
	service.Shutdown()
}

func TestAppHandlerHandleStreamUnavailable(t *testing.T) {
	handler := NewAppHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/app", nil)
	recorder := httptest.NewRecorder()

	handler.HandleStream(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected non-stream error response, got Content-Type %q", got)
	}
}
