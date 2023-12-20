package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

func SetupGlobalMiddleware(handler http.Handler) http.Handler {
	next := cors(handler)
	next = middleware.RequestID(next)
	next = middleware.Logger(next)
	next = middleware.Recoverer(next)

	if authToken != nil {
		next = TokenAuth("restricted", authToken.Token)(next)
	}

	if basicAuth != nil {
		next = middleware.BasicAuth(
			"restricted", map[string]string{basicAuth.Username: basicAuth.Password},
		)(next)
	}
	next = prefixChecker(next)

	return next
}

var (
	defaultHandler http.Handler
	basicAuth      *BasicAuth
	authToken      *AuthToken
)

type Options struct {
	Handler   http.Handler
	BasicAuth *BasicAuth
	AuthToken *AuthToken
}

type BasicAuth struct {
	Username string
	Password string
}

type AuthToken struct {
	Token string
}

func Setup(opts *Options) {
	defaultHandler = opts.Handler
	basicAuth = opts.BasicAuth
	authToken = opts.AuthToken
}

func prefixChecker(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
			} else {
				defaultHandler.ServeHTTP(w, r)
			}
		})
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Access-Control-Allow-Origin", "*")
			w.Header().Add("Access-Control-Allow-Methods", "*")
			w.Header().Add("Access-Control-Allow-Headers", "*")
			if r.Method == http.MethodOptions {
				return
			}
			h.ServeHTTP(w, r)
		})
}
