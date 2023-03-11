package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func ConfigRoutes(r *chi.Mux) {
	r.Get("/", HandleGetList())

	r.Post("/", HandlePostList())

	r.Route("/dags", func(r chi.Router) {
		r.Get("/", HandleGetList())
		r.Post("/", HandlePostList())

		dagRoute := func(r chi.Router) {
			r.Use(dagContext)
			r.Use(tabContext)
			r.Get("/", HandleGetDAG())
			r.Post("/", HandlePostDAG())
			r.Delete("/", HandleDeleteDAG())
		}

		r.Route("/{dagName}", dagRoute)
		r.Route("/{dagName}/{tabName}", dagRoute)
	})

	r.Get("/search", HandleGetSearch())
	r.Get("/assets/*", HandleGetAssets())
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
