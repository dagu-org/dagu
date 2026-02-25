package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLicenseClaims_WarningCode(t *testing.T) {
	t.Parallel()

	t.Run("round-trips when set", func(t *testing.T) {
		t.Parallel()
		claims := &LicenseClaims{
			Plan:        "pro",
			Features:    []string{FeatureAudit},
			WarningCode: "MACHINE_LIMIT_EXCEEDED",
		}
		assert.Equal(t, "MACHINE_LIMIT_EXCEEDED", claims.WarningCode)
	})

	t.Run("defaults to empty string when absent", func(t *testing.T) {
		t.Parallel()
		claims := &LicenseClaims{
			Plan:     "pro",
			Features: []string{FeatureAudit},
		}
		assert.Equal(t, "", claims.WarningCode)
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
