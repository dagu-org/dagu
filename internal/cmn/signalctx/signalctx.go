// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package signalctx

import "context"

type osSignalsDisabledKey struct{}

// WithOSSignalsDisabled marks a context so in-process test helpers can skip
// subscribing to process-wide OS signals.
func WithOSSignalsDisabled(ctx context.Context) context.Context {
	return context.WithValue(ctx, osSignalsDisabledKey{}, true)
}

// OSSignalsDisabled reports whether OS signal subscriptions should be skipped.
func OSSignalsDisabled(ctx context.Context) bool {
	disabled, _ := ctx.Value(osSignalsDisabledKey{}).(bool)
	return disabled
}
