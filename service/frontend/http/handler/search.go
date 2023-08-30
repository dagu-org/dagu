package handlers

import (
	"net/http"
)

func handleGetSearch() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "search")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}
