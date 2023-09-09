package server

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (svr *Server) defaultRoutes(r *chi.Mux) *chi.Mux {
	r.Get("/assets/*", svr.handleGetAssets())
	r.Get("/*", svr.handleRequest())

	return r
}

func (svr *Server) handleRequest() http.HandlerFunc {
	renderFunc := svr.useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}

func (svr *Server) handleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(svr.assets)).ServeHTTP(w, r)
	}
}
