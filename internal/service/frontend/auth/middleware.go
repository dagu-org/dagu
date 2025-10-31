package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	oidc "github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

// Options configures the authentication middleware.
type Options struct {
	Realm            string
	APITokenEnabled  bool
	APIToken         string
	BasicAuthEnabled bool
	OIDCAuthEnabled  bool
	OIDCProvider     *oidc.Provider
	OIDCVerify       *oidc.IDTokenVerifier
	OIDCConfig       *oauth2.Config
	OIDCWhitelist    []string
	Creds            map[string]string
}

// DefaultOptions provides sensible defaults for the middleware.
func DefaultOptions() Options {
	return Options{
		Realm:            "Restricted",
		APITokenEnabled:  false,
		BasicAuthEnabled: false,
		OIDCAuthEnabled:  false,
	}
}

// Middleware creates an HTTP middleware for authentication.
func Middleware(opts Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no auth is enabled, skip authentication
			if !opts.BasicAuthEnabled && !opts.APITokenEnabled && !opts.OIDCAuthEnabled {
				next.ServeHTTP(w, r)
				return
			}

			// Try API token authentication first if enabled
			if opts.APITokenEnabled && checkAPIToken(r, opts.APIToken) {
				next.ServeHTTP(w, r)
				return
			}

			// Try Basic Auth if enabled
			if opts.BasicAuthEnabled {
				if user, pass, ok := r.BasicAuth(); ok {
					if checkBasicAuth(user, pass, opts.Creds) {
						next.ServeHTTP(w, r)
						return
					}
				}

				// Unauthorized: require basic auth
				requireBasicAuth(w, opts.Realm)
				return
			}

			// Try OIDC Auth if enabled
			if opts.OIDCAuthEnabled {
				checkOIDCToken(next, opts.OIDCVerify, w, r)
				return
			}

			// If API token auth was tried and failed, and basic auth is not enabled
			requireBearerAuth(w, opts.Realm)
		})
	}
}

func OIDCMiddleware(opts Options) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkOIDCAuth(next, opts.OIDCProvider, opts.OIDCVerify, opts.OIDCConfig, opts.OIDCWhitelist, w, r)
		})
	}
}

// checkAPIToken validates the API token from the Authorization header.
func checkAPIToken(r *http.Request, validToken string) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}

	// Use constant time comparison to prevent timing attacks
	token := strings.TrimPrefix(authHeader, "Bearer ")
	return subtle.ConstantTimeCompare([]byte(token), []byte(validToken)) == 1
}

// checkBasicAuth validates the username and password.
func checkBasicAuth(user, pass string, validCreds map[string]string) bool {
	credPass, credUserOk := validCreds[user]
	if !credUserOk {
		return false
	}

	// Use constant time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(pass), []byte(credPass)) == 1
}

// requireBasicAuth sends a 401 response with Basic auth challenge.
func requireBasicAuth(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}

// requireBearerAuth sends a 401 response with Bearer auth challenge.
func requireBearerAuth(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}
