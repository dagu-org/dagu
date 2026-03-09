// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package license

// Feature constants for licensed capabilities.
const (
	FeatureAudit = "audit"
	FeatureRBAC  = "rbac"
	FeatureSSO   = "sso"
)

// Checker provides license status information.
type Checker interface {
	IsFeatureEnabled(feature string) bool
	Plan() string
	IsGracePeriod() bool
	IsCommunity() bool
	Claims() *LicenseClaims
	WarningCode() string
}
