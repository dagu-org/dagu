package admin

import (
	"net/http"
	"regexp"
)

type adminHandler struct {
	config *Config
	routes map[string]map[*regexp.Regexp]http.HandlerFunc
}

func newAdminHandler(cfg *Config, routes []*route) *adminHandler {
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
