package license

import (
	"slices"

	"github.com/golang-jwt/jwt/v5"
)

// LicenseClaims represents the custom JWT claims for a Dagu license.
type LicenseClaims struct {
	jwt.RegisteredClaims

	ClaimsVersion int      `json:"cv"`
	Plan          string   `json:"plan"`
	Features      []string `json:"features"`
	ActivationID  string   `json:"activation_id"`
	WarningCode   string   `json:"wc,omitempty"`
}

// HasFeature returns true if the given feature is included in the license claims.
func (c *LicenseClaims) HasFeature(name string) bool {
	return slices.Contains(c.Features, name)
}
