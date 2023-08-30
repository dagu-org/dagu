package handlers

import (
	"net/http"
)

func handleGetDAG() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "dag")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}
