// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package signalctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOSSignalsDisabled(t *testing.T) {
	base := context.Background()
	require.False(t, OSSignalsDisabled(base))
	require.True(t, OSSignalsDisabled(WithOSSignalsDisabled(base)))
}
