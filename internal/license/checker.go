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
