// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeWebhookForwardHeader(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "x-github-event", NormalizeWebhookForwardHeader(" X-GitHub-Event "))
	assert.Equal(t, "", NormalizeWebhookForwardHeader("   "))
}

func TestIsDeniedWebhookForwardHeader(t *testing.T) {
	t.Parallel()

	assert.True(t, IsDeniedWebhookForwardHeader("Authorization"))
	assert.True(t, IsDeniedWebhookForwardHeader(" authorization "))
	assert.False(t, IsDeniedWebhookForwardHeader("X-GitHub-Event"))
}
