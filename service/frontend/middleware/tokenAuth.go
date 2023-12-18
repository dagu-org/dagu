package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// TokenAuth implements a similar middleware handler like go-chi's BasicAuth middleware but for bearer tokens
func TokenAuth(realm string, token string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.Split(r.Header.Get("Authorization"), " ")
			if len(authHeader) < 2 {
				tokenAuthFailed(w, realm)
				return
			}

			bearer := authHeader[1]
			if bearer == "" {
				tokenAuthFailed(w, realm)
				return
			}

			if subtle.ConstantTimeCompare([]byte(bearer), []byte(token)) != 1 {
				tokenAuthFailed(w, realm)
				return
			}

			next.ServeHTTP(w, r)

		})
	}
}

func tokenAuthFailed(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}
