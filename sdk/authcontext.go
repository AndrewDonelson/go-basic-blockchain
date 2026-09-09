// Package sdk is a software development kit for building blockchain applications.
// File sdk/authcontext.go - request principal propagation
package sdk

import "context"

// principalContextKey is an unexported type so no other package can collide with
// or forge this context key.
type principalContextKey struct{}

// withPrincipal returns a context carrying the authenticated principal.
//
// The middleware previously built a context and then passed the *original*
// unchanged context to the next handler, so downstream code had no way to learn
// who the caller was.
func withPrincipal(ctx context.Context, principal string) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns the authenticated principal for a request, if any.
func PrincipalFromContext(ctx context.Context) (string, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(string)
	return principal, ok && principal != ""
}
