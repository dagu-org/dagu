package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

func BasicAuth(realm string, creds map[string]string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.Split(r.Header.Get("Authorization"), " ")
			if skipBasicAuth(authHeader) {
				next.ServeHTTP(w, r)
				return
			}
			user, pass, ok := r.BasicAuth()
			if !ok {
				basicAuthFailed(w, realm)
				return
			}

			credPass, credUserOk := creds[user]
			if !credUserOk || subtle.ConstantTimeCompare([]byte(pass), []byte(credPass)) != 1 {
				basicAuthFailed(w, realm)
				return
			}

			next.ServeHTTP(w, r.WithContext(withAuthenticated(r.Context())))
		})
	}
}

// skipBasicAuth skips basic auth middleware when the auth token is set
func skipBasicAuth(authHeader []string) bool {
	return authToken != nil &&
		len(authHeader) >= 2 &&
		authHeader[0] == "Bearer"
}

func basicAuthFailed(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}
