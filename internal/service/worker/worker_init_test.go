// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package worker

import (
	"testing"

	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/stretchr/testify/require"
)

func TestNewWorker_UsesTaskHandlerInitializer(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Paths: config.PathsConfig{
			BaseConfig: "/tmp/base.yaml",
		},
	}

	w := NewWorker("test-worker", 1, nil, nil, cfg)
	require.NotNil(t, w)

	handler, ok := w.handler.(*taskHandler)
	require.True(t, ok, "expected NewWorker to install the default task handler")
	require.Equal(t, cfg.Paths.BaseConfig, handler.baseConfig)
	require.NotNil(t, handler.subCmdBuilder)
}
