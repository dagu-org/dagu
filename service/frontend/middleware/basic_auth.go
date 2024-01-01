package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

func BasicAuth(realm string, creds map[string]string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.Split(r.Header.Get("Authorization"), " ")
			if skipBasicAuth(authHeader) {
				next.ServeHTTP(w, r)
				return
			}
			middleware.BasicAuth("restricted", creds)(next).ServeHTTP(w, r)
		})
	}
}

// skipBasicAuth skips basic auth middleware when the auth token is set
func skipBasicAuth(authHeader []string) bool {
	return authToken != nil &&
		len(authHeader) >= 2 &&
		authHeader[0] == "Bearer"
}
