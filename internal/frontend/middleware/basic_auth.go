// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

const (
	authHeaderKey = "Authorization"
)

// nolint:revive
func BasicAuth(realm string, creds map[string]string) func(
	next http.Handler,
) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.Split(r.Header.Get(authHeaderKey), " ")
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
