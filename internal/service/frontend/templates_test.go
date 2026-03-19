// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"strings"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/assert"
)

func resetAssetVersionCache() {
	assetVersion = ""
	assetVersionOnce = sync.Once{}
}

func TestCurrentAssetVersionUsesBundleHashForDevBuilds(t *testing.T) {
	originalVersion := config.Version
	t.Cleanup(func() {
		config.Version = originalVersion
		resetAssetVersionCache()
	})

	config.Version = "0.0.0"
	resetAssetVersionCache()

	got := currentAssetVersion()

	assert.True(t, strings.HasPrefix(got, "0.0.0-"))
	assert.Greater(t, len(got), len("0.0.0-"))
}

func TestCurrentAssetVersionUsesReleaseVersionWhenSet(t *testing.T) {
	originalVersion := config.Version
	t.Cleanup(func() {
		config.Version = originalVersion
		resetAssetVersionCache()
	})

	config.Version = "1.2.3"
	resetAssetVersionCache()

	assert.Equal(t, "1.2.3", currentAssetVersion())
}
