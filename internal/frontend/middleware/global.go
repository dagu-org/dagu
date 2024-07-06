package middleware

import (
	"context"
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

	if authBasic != nil {
		next = BasicAuth(
			"restricted",
			map[string]string{authBasic.Username: authBasic.Password},
		)(next)
	}
	next = prefixChecker(next)

	return next
}

type authCtxKey struct{}

type authCtx struct {
	authenticated bool
}

func withAuthenticated(ctx context.Context) context.Context {
	return context.WithValue(ctx, authCtxKey{}, &authCtx{authenticated: true})
}

func isAuthenticated(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	auth, ok := ctx.Value(authCtxKey{}).(*authCtx)
	return ok && auth.authenticated
}

var (
	defaultHandler http.Handler
	authBasic      *AuthBasic
	authToken      *AuthToken
)

type Options struct {
	Handler   http.Handler
	AuthBasic *AuthBasic
	AuthToken *AuthToken
}

type AuthBasic struct {
	Username string
	Password string
}

type AuthToken struct {
	Token string
}

func Setup(opts *Options) {
	defaultHandler = opts.Handler
	authBasic = opts.AuthBasic
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
