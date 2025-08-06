package digraph

// AuthConfig represents Docker registry authentication configuration.
// This is a simplified structure for user convenience that will be
// converted to Docker's registry.AuthConfig format when needed.
type AuthConfig struct {
	// Username for registry authentication
	Username string `json:"username,omitempty"`
	// Password for registry authentication  
	Password string `json:"password,omitempty"`
	// Auth can be used instead of username/password for pre-encoded credentials
	// This should be base64(username:password)
	Auth string `json:"auth,omitempty"`
}