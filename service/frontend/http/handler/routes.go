package handlers

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
)

func ConfigRoutes(r *chi.Mux) *chi.Mux {
	r.Get("/", handleIndex())

	r.Route("/dags", func(r chi.Router) {
		r.Get("/", handleIndex())

		dagRoute := func(r chi.Router) {
			r.Get("/", handleGetDAG())
		}

		r.Route("/{dagName}", dagRoute)
		r.Route("/{dagName}/{tabName}", dagRoute)
	})

	r.Get("/search", handleGetSearch())
	r.Get("/assets/*", handleGetAssets())

	return r
}

func handleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(assetsFS)).ServeHTTP(w, r)
	}
}

func renderJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Printf("%v", err)
	}
}
