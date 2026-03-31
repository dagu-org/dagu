// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventstore

import (
	"context"

	"github.com/dagu-org/dagu/internal/core/exec"
)

type contextKey struct{}

type contextValue struct {
	service *Service
	source  Source
}

func WithContext(ctx context.Context, service *Service, source Source) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if service == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, contextValue{
		service: service,
		source:  normalizeSource(source),
	})
}

func FromContext(ctx context.Context) (*Service, Source, bool) {
	if ctx == nil {
		return nil, Source{}, false
	}
	v, ok := ctx.Value(contextKey{}).(contextValue)
	if !ok || v.service == nil {
		return nil, Source{}, false
	}
	return v.service, v.source, true
}

func SourceFromContext(ctx context.Context) (Source, bool) {
	_, source, ok := FromContext(ctx)
	return source, ok
}

func EmitPersistedStatusFromContext(ctx context.Context, status *exec.DAGRunStatus) error {
	service, source, ok := FromContext(ctx)
	if !ok || service == nil || status == nil {
		return nil
	}
	eventType, ok := PersistedDAGRunEventTypeForStatus(status.Status)
	if !ok {
		return nil
	}
	return service.Emit(context.WithoutCancel(ctx), NewDAGRunEvent(source, eventType, status, nil))
}
