package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/service/frontend/api/pathutil"
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
	PublicPaths      []string
	// JWTValidator validates JWT tokens for builtin auth mode.
	// When set, JWT Bearer tokens are accepted as an authentication method.
	JWTValidator TokenValidator
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
// It supports multiple authentication methods simultaneously:
// - JWT Bearer tokens (if JWTValidator is set)
// - Static API tokens (if APITokenEnabled)
// - HTTP Basic Auth (if BasicAuthEnabled)
// - OIDC (if OIDCAuthEnabled)
// All configured methods work at the same time.
func Middleware(opts Options) func(next http.Handler) http.Handler {
	publicPaths := make(map[string]struct{}, len(opts.PublicPaths))
	for _, p := range opts.PublicPaths {
		publicPaths[pathutil.NormalizePath(p)] = struct{}{}
	}

	jwtEnabled := opts.JWTValidator != nil
	anyAuthEnabled := opts.BasicAuthEnabled || opts.APITokenEnabled || opts.OIDCAuthEnabled || jwtEnabled

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow unauthenticated access to explicitly configured public paths.
			if _, ok := publicPaths[pathutil.NormalizePath(r.URL.Path)]; ok {
				next.ServeHTTP(w, r)
				return
			}

			// If no auth is enabled, skip authentication
			if !anyAuthEnabled {
				next.ServeHTTP(w, r)
				return
			}

			// Try JWT token authentication if enabled (for builtin auth mode)
			if jwtEnabled {
				if token := extractBearerToken(r); token != "" {
					user, err := opts.JWTValidator.GetUserFromToken(r.Context(), token)
					if err == nil {
						// JWT token valid - inject user into context
						ctx := auth.WithUser(r.Context(), user)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					// JWT validation failed, continue to try other methods
				}
			}

			// Try API token authentication if enabled
			if opts.APITokenEnabled && checkAPIToken(r, opts.APIToken) {
				// API token grants full admin access
				// Inject a synthetic admin user so that permission checks (requireWrite,
				// requireExecute, requireAdmin) work correctly in builtin auth mode
				adminUser := &auth.User{
					ID:       "api-token",
					Username: "api-token",
					Role:     auth.RoleAdmin,
				}
				ctx := auth.WithUser(r.Context(), adminUser)
				next.ServeHTTP(w, r.WithContext(ctx))
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
			}

			// Try OIDC Auth if enabled
			if opts.OIDCAuthEnabled {
				checkOIDCToken(next, opts.OIDCVerify, w, r)
				return
			}

			// No valid authentication found - send appropriate challenge
			if opts.BasicAuthEnabled {
				requireBasicAuth(w, opts.Realm)
				return
			}
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
