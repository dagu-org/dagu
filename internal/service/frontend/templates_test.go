// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/assert"
)

func resetAssetVersionCache() {
	assetVersion = ""
	assetVersionOnce = sync.Once{}
}

func TestFormatAssetVersionUsesBundleHashForDevBuilds(t *testing.T) {
	bundle := []byte("bundle")
	sum := sha256.Sum256(bundle)
	want := "0.0.0-" + hex.EncodeToString(sum[:8])

	assert.Equal(t, want, formatAssetVersion("0.0.0", bundle))
}

func TestFormatAssetVersionSupportsEmptyVersion(t *testing.T) {
	bundle := []byte("bundle")
	sum := sha256.Sum256(bundle)
	want := hex.EncodeToString(sum[:8])

	assert.Equal(t, want, formatAssetVersion("", bundle))
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
