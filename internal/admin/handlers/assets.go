package handlers

import (
	"net/http"
	"strings"
)

func HandleGetAssets(pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		r.URL.Path = pathPrefix + r.URL.Path
		w.Header().Set("Cache-Control", "max-age=86400")
		http.FileServer(http.FS(assets)).ServeHTTP(w, r)
	}
}
