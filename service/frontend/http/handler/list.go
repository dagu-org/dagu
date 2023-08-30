package handlers

import (
	"net/http"
)

func handleIndex() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}
