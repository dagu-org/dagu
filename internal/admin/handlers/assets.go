package handlers

import (
	"bufio"
	"io"
	"net/http"
	"path"
	"regexp"
)

var assetsPath = "web/assets/"
var (
	jsRe  = regexp.MustCompile(`^/assets/(js/.*.js)?.*$`)
	cssRe = regexp.MustCompile(`^/assets/(css/.*.css)?.*$`)
)

type AssetType int

const (
	AssetTypeJs AssetType = iota
	AssetTypeCss
)

func HandleGetAssets(assetType AssetType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var m = ""
		switch assetType {
		case AssetTypeJs:
			m = getMatch(r, jsRe)
			w.Header().Set("Content-Type", "application/javascript")
		case AssetTypeCss:
			m = getMatch(r, cssRe)
			w.Header().Set("Content-Type", "text/css")
		}
		if m == "" {
			http.NotFound(w, r)
			return
		}
		f, err := assets.Open(path.Join(assetsPath, m))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		_, err = io.Copy(w, bufio.NewReader(f))
		if err != nil {
			encodeError(w, err)
		}
	}
}

func getMatch(r *http.Request, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(r.URL.Path)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
