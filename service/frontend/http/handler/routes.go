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

func ConfigRoutes(r *chi.Mux) {
	r.Get("/", handleGetList())

	r.Post("/", handlePostList())

	r.Route("/dags", func(r chi.Router) {
		r.Get("/", handleGetList())
		r.Post("/", handlePostList())

		dagRoute := func(r chi.Router) {
			r.Use(dagContext)
			r.Use(tabContext)
			r.Get("/", handleGetDAG())
			r.Post("/", handlePostDAG())
			r.Delete("/", handleDeleteDAG())
		}

		r.Route("/{dagName}", dagRoute)
		r.Route("/{dagName}/{tabName}", dagRoute)
	})

	r.Get("/search", handleGetSearch())
	r.Get("/assets/*", handleGetAssets())
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

func tabContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := chi.URLParam(r, "tabName")
		if s == "" {
			s = dag_TabType_Status
		}
		ctx := context.WithValue(r.Context(), ctxKeyTab{}, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tabNameFromCtx(ctx context.Context) string {
	return ctx.Value(ctxKeyTab{}).(string)
}

func nameWithExt(name string) string {
	s := strings.TrimSuffix(name, ".yaml")
	return fmt.Sprintf("%s.yaml", s)
}
