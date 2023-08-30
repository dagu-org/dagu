package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func ConfigRoutes(r *chi.Mux) *chi.Mux {
	r.Get("/", handleIndex())

	r.Route("/dags", func(r chi.Router) {
		r.Get("/", handleIndex())

		dagRoute := func(r chi.Router) {
			r.Use(dagContext)
			r.Get("/", handleGetDAG())
			r.Delete("/", handleDeleteDAG())
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

type ctxKeyDAG struct{}

func dagContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := chi.URLParam(r, "dagName")
		if s == "" {
			encodeError(w, errInvalidArgs)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyDAG{}, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func dagNameFromCtx(ctx context.Context) string {
	return ctx.Value(ctxKeyDAG{}).(string)
}

type ctxKeyTab struct{}

func nameWithExt(name string) string {
	s := strings.TrimSuffix(name, ".yaml")
	return fmt.Sprintf("%s.yaml", s)
}
