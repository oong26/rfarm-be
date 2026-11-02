package middleware

import (
	"context"
	"sync"
)

type principalContextKey struct{}
type requestStateKey struct{}

// Principal is the authenticated identity placed in the request context.
// The future auth middleware fills it from the access token.
type Principal struct {
	UserID string
	FarmID string
	Role   string
}

// requestState lets the outer access log see the identity that an inner auth
// middleware sets. Context values flow inward only, so a small mutable holder
// bridges the layers.
type requestState struct {
	mu     sync.Mutex
	userID string
	farmID string
}

func (s *requestState) set(p Principal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userID = p.UserID
	s.farmID = p.FarmID
}

func (s *requestState) identity() (userID, farmID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.userID, s.farmID
}

// WithPrincipal stores the authenticated identity for downstream handlers and
// for the access log.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok {
		state.set(principal)
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFrom returns the authenticated identity.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

// FarmIDFrom is the only supported way for a handler to read the farm scope.
func FarmIDFrom(ctx context.Context) string {
	principal, _ := PrincipalFrom(ctx)
	return principal.FarmID
}

// UserIDFrom returns the authenticated user, or "" when unauthenticated.
func UserIDFrom(ctx context.Context) string {
	principal, _ := PrincipalFrom(ctx)
	return principal.UserID
}
