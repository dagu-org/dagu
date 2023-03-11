package handlers

import (
	"net/http"
)

func HandleGetAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/web" + r.URL.Path
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(assets)).ServeHTTP(w, r)
	}
}
