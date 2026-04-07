// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"testing"

	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/stretchr/testify/require"
)

func TestRegisteredPortRespectsDisabledHealthServer(t *testing.T) {
	t.Parallel()

	s := &Scheduler{
		config: &config.Config{
			Scheduler: config.Scheduler{
				Port: 8090,
			},
		},
	}

	require.Equal(t, 8090, s.registeredPort())
	s.DisableHealthServer()
	require.Equal(t, 0, s.registeredPort())
}
