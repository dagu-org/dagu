package admin

import (
	"net/http"
	"regexp"

	"github.com/yohamta/dagu/internal/config"
)

type adminHandler struct {
	config *config.Config
	routes map[string]map[*regexp.Regexp]http.HandlerFunc
}

func newAdminHandler(cfg *config.Config, routes []*route) *adminHandler {
	hdl := &adminHandler{
		config: cfg,
		routes: map[string]map[*regexp.Regexp]http.HandlerFunc{},
	}
	hdl.configure(routes)
	return hdl
}

func (hdl *adminHandler) configure(routes []*route) {
	for _, route := range routes {
		hdl.addRoute(route.method, route.pattern, route.handler)
	}
}

func (hdl *adminHandler) addRoute(method, pattern string, handler http.HandlerFunc) {
	if _, ok := hdl.routes[method]; !ok {
		hdl.routes[method] = map[*regexp.Regexp]http.HandlerFunc{}
	}
	hdl.routes[method][regexp.MustCompile(pattern)] = handler
}

func (hdl *adminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if patterns, ok := hdl.routes[r.Method]; ok {
		for re, handler := range patterns {
			if re.MatchString(r.URL.Path) {
				handler(w, r)
				return
			}
		}
	}
	encodeError(w, errNotFound)
}
