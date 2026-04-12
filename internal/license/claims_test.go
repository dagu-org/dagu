// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package license

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseClaims_WarningCode(t *testing.T) {
	t.Parallel()

	t.Run("round-trips through JSON when set", func(t *testing.T) {
		t.Parallel()
		original := &LicenseClaims{
			Plan:        "pro",
			Features:    []string{FeatureAudit},
			WarningCode: "MACHINE_LIMIT_EXCEEDED",
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"wc"`)

		var decoded LicenseClaims
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, "MACHINE_LIMIT_EXCEEDED", decoded.WarningCode)
	})

	t.Run("omitted from JSON when empty", func(t *testing.T) {
		t.Parallel()
		original := &LicenseClaims{
			Plan:     "pro",
			Features: []string{FeatureAudit},
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"wc"`)
	})
}

func TestLicenseClaims_GraceDays(t *testing.T) {
	t.Parallel()

	t.Run("round-trips zero grace days through JSON", func(t *testing.T) {
		t.Parallel()
		zero := 0
		original := &LicenseClaims{
			Plan:      "trial",
			Features:  []string{FeatureAudit},
			GraceDays: &zero,
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"grace_days":0`)

		var decoded LicenseClaims
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.NotNil(t, decoded.GraceDays)
		assert.Equal(t, 0, *decoded.GraceDays)
	})
}

func TestLicenseClaims_HasFeature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features []string
		input    string
		want     bool
	}{
		{
			name:     "feature present returns true",
			features: []string{"audit", "rbac"},
			input:    "audit",
			want:     true,
		},
		{
			name:     "feature absent returns false",
			features: []string{"rbac", "sso"},
			input:    "audit",
			want:     false,
		},
		{
			name:     "empty features slice returns false",
			features: []string{},
			input:    FeatureAudit,
			want:     false,
		},
		{
			name:     "nil features slice returns false",
			features: nil,
			input:    FeatureAudit,
			want:     false,
		},
		{
			name:     "feature lookup is case-sensitive uppercase does not match lowercase",
			features: []string{FeatureAudit},
			input:    "AUDIT",
			want:     false,
		},
		{
			name:     "FeatureAudit constant present returns true",
			features: []string{FeatureAudit, FeatureRBAC, FeatureSSO},
			input:    FeatureAudit,
			want:     true,
		},
		{
			name:     "FeatureRBAC constant present returns true",
			features: []string{FeatureAudit, FeatureRBAC, FeatureSSO},
			input:    FeatureRBAC,
			want:     true,
		},
		{
			name:     "FeatureSSO constant present returns true",
			features: []string{FeatureAudit, FeatureRBAC, FeatureSSO},
			input:    FeatureSSO,
			want:     true,
		},
		{
			name:     "unknown feature string returns false",
			features: []string{FeatureAudit, FeatureRBAC, FeatureSSO},
			input:    "unknown-feature",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			claims := &LicenseClaims{
				Features: tc.features,
			}
			got := claims.HasFeature(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
