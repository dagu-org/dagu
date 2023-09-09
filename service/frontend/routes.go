package frontend

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func ConfigRoutes(r *chi.Mux) *chi.Mux {
	r.Get("/assets/*", handleGetAssets())
	r.Get("/*", handleRequest())

	return r
}

func handleRequest() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}

func handleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(assetsFS)).ServeHTTP(w, r)
	}
}
