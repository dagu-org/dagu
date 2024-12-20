// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package middleware

import "net/http"

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appLogger.Infof("Request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
