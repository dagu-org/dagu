package auth

import "context"

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

type authCtxKey struct{}

type authCtx struct {
	authenticated bool
}
