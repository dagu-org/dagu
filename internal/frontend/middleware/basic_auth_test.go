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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicAuth(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter,
		_ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		require.NoError(t, err)
	})
	fakeAuthHeader := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikpva"
	fakeAuthToken := AuthToken{
		Token: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikpva",
	}
	testCase := []struct {
		name       string
		authHeader string
		authToken  *AuthToken
		httpStatus int
	}{
		{
			name:       "auth header set, auth token set",
			authHeader: fakeAuthHeader,
			authToken:  &fakeAuthToken,
			httpStatus: http.StatusOK,
		},
		{
			name:       "auth header set, auth token unset",
			authHeader: fakeAuthHeader,
			authToken:  nil,
			httpStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth header unset, auth token set",
			authHeader: "",
			authToken:  &fakeAuthToken,
			httpStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth header unset, auth token unset",
			authHeader: "",
			authToken:  nil,
			httpStatus: http.StatusUnauthorized,
		},
	}
	// incorrectCreds triggers HTTP 401 Unauthorized upon basic auth
	incorrectCreds := map[string]string{
		"INCORRECT_USERNAME": "INCORRECT_PASSWORD",
	}
	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			r, err := http.NewRequest("GET", "/test", nil)
			require.NoError(t, err)

			r.Header.Add("Authorization", tc.authHeader)

			w := httptest.NewRecorder()
			authToken = tc.authToken

			BasicAuth("restricted", incorrectCreds)(testHandler).ServeHTTP(w, r)

			res := w.Result()
			defer func() {
				_ = res.Body.Close()
			}()
			require.Equal(t, tc.httpStatus, res.StatusCode)
		})
	}
}
