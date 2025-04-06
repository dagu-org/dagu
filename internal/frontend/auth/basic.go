package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

func Basic(realm string, creds map[string]string) func(
	next http.Handler,
) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAuthenticated(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			user, pass, ok := r.BasicAuth()
			if !ok {
				basicAuthFailed(w, realm)
				return
			}

			credPass, credUserOk := creds[user]
			if !credUserOk || subtle.ConstantTimeCompare(
				[]byte(pass),
				[]byte(credPass),
			) != 1 {
				basicAuthFailed(w, realm)
				return
			}

			next.ServeHTTP(w, r.WithContext(withAuthenticated(r.Context())))
		})
	}
}

func basicAuthFailed(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}
