// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import "strings"

const webhookAuthorizationHeader = "authorization"

// NormalizeWebhookForwardHeader canonicalizes a webhook header name for
// config validation and runtime matching.
func NormalizeWebhookForwardHeader(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// IsDeniedWebhookForwardHeader reports whether a webhook header must never be
// forwarded into DAG runtime params.
func IsDeniedWebhookForwardHeader(name string) bool {
	return NormalizeWebhookForwardHeader(name) == webhookAuthorizationHeader
}
