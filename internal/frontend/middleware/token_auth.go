// Copyright (C) 2024 The Dagu Authors
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

// TokenAuth implements a similar middleware handler like go-chi's BasicAuth
// middleware but for bearer tokens
func TokenAuth(
	realm string, token string,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipTokenAuth(*r) {
				next.ServeHTTP(w, r)
				return
			}

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

func skipTokenAuth(r http.Request) bool {
	return isAuthenticated(r.Context())
}

func tokenAuthFailed(w http.ResponseWriter, realm string) {
	w.Header().Add("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}
