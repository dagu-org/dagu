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

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (svr *Server) defaultRoutes(r *chi.Mux) *chi.Mux {
	r.Get("/assets/*", svr.handleGetAssets())
	r.Get("/*", svr.handleRequest())

	return r
}

func (svr *Server) handleRequest() http.HandlerFunc {
	renderFunc := svr.useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, _ *http.Request) {
		renderFunc(w, nil)
	}
}

func (svr *Server) handleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(svr.assets)).ServeHTTP(w, r)
	}
}
