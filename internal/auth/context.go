package auth

import "context"

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// userContextKey is the key for storing the authenticated user in context.
	userContextKey contextKey = "auth_user"
	// clientIPContextKey is the key for storing the client IP address in context.
	clientIPContextKey contextKey = "client_ip"
)

// WithUser returns a new context that carries the provided user value.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context.
// It returns the user and true if a *User value is present for the package's userContextKey, or nil and false otherwise.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

// WithClientIP returns a new context that carries the client IP address.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPContextKey, ip)
}

// ClientIPFromContext retrieves the client IP address from the context.
// It returns the IP address and true if present, or empty string and false otherwise.
func ClientIPFromContext(ctx context.Context) (string, bool) {
	ip, ok := ctx.Value(clientIPContextKey).(string)
	return ip, ok
}
