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
	// PublicPathPrefixes are path prefixes that bypass authentication.
	// Any path starting with one of these prefixes will be allowed without auth.
	PublicPathPrefixes []string
	// JWTValidator validates JWT tokens for builtin auth mode.
	// When set, JWT Bearer tokens are accepted as an authentication method.
	JWTValidator TokenValidator
	// APIKeyValidator validates standalone API keys with roles.
	// When set, API keys with the "dagu_" prefix are accepted as an authentication method.
	APIKeyValidator APIKeyValidator
	// AuthRequired indicates whether authentication is required.
	// When false (e.g., auth mode "none"), credentials are validated if provided
	// but unauthenticated requests are allowed through.
	AuthRequired bool
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

// ClientIPMiddleware creates an HTTP middleware that adds the client IP to the request context.
// This should be applied before authentication middleware to ensure IP is available for audit logging.
func ClientIPMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithClientIP(r.Context(), getClientIP(r))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
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

	// Process public path prefixes - ensure they have leading slash but preserve trailing slash
	// The trailing slash is important for prefixes: "/api/v2/webhooks/" should only match
	// paths like "/api/v2/webhooks/foo", not "/api/v2/webhooks" itself
	publicPrefixes := make([]string, 0, len(opts.PublicPathPrefixes))
	for _, p := range opts.PublicPathPrefixes {
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		publicPrefixes = append(publicPrefixes, p)
	}

	jwtEnabled := opts.JWTValidator != nil
	apiKeyEnabled := opts.APIKeyValidator != nil
	anyAuthEnabled := opts.BasicAuthEnabled || opts.APITokenEnabled || opts.OIDCAuthEnabled || jwtEnabled || apiKeyEnabled

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			normalizedPath := pathutil.NormalizePath(r.URL.Path)

			// Allow unauthenticated access to explicitly configured public paths.
			if _, ok := publicPaths[normalizedPath]; ok {
				next.ServeHTTP(w, r)
				return
			}

			// Allow unauthenticated access to paths matching public prefixes.
			for _, prefix := range publicPrefixes {
				if strings.HasPrefix(normalizedPath, prefix) {
					next.ServeHTTP(w, r)
					return
				}
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
						// JWT token valid - inject user and client IP into context
						ctx := auth.WithUser(r.Context(), user)
						ctx = auth.WithClientIP(ctx, getClientIP(r))
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					// JWT validation failed, continue to try other methods
				}
			}

			// Try standalone API key authentication if enabled
			// API keys have the "dagu_" prefix and have their own role assignment
			if apiKeyEnabled {
				if token := extractBearerToken(r); token != "" && strings.HasPrefix(token, "dagu_") {
					apiKey, err := opts.APIKeyValidator.ValidateAPIKey(r.Context(), token)
					if err == nil {
						// API key valid - create synthetic user with the key's role
						syntheticUser := &auth.User{
							ID:       "apikey:" + apiKey.ID,
							Username: "apikey:" + apiKey.Name,
							Role:     apiKey.Role,
						}
						ctx := auth.WithUser(r.Context(), syntheticUser)
						ctx = auth.WithClientIP(ctx, getClientIP(r))
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					// API key validation failed, continue to try other methods
				}
			}

			// Try static API token authentication if enabled
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
				ctx = auth.WithClientIP(ctx, getClientIP(r))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try Basic Auth if enabled
			if opts.BasicAuthEnabled {
				if user, pass, ok := r.BasicAuth(); ok {
					// Credentials were provided - must validate
					if checkBasicAuth(user, pass, opts.Creds) {
						// Create user and add to context
						basicUser := &auth.User{
							ID:       user,
							Username: user,
							Role:     auth.RoleAdmin,
						}
						ctx := auth.WithUser(r.Context(), basicUser)
						ctx = auth.WithClientIP(ctx, getClientIP(r))
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					// Invalid credentials - always reject
					requireBasicAuth(w, opts.Realm)
					return
				}
			}

			// Try OIDC Auth if enabled
			if opts.OIDCAuthEnabled {
				checkOIDCToken(next, opts.OIDCVerify, w, r)
				return
			}

			// No credentials provided
			// If auth is not required (e.g., mode "none"), allow the request through
			if !opts.AuthRequired {
				next.ServeHTTP(w, r)
				return
			}

			// Auth is required - send appropriate challenge
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

// getClientIP extracts the client IP address from the request.
// It checks X-Forwarded-For and X-Real-IP headers for proxied requests.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
