// Package middleware contains the HTTP middleware shared by every route:
// request IDs, panic recovery, access logging, CORS and farm scope.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"
)

// HeaderRequestID carries the correlation ID in the request and response.
const HeaderRequestID = "X-Request-ID"

type requestIDContextKey struct{}

var (
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	fallbackID       atomic.Uint64
)

// RequestID reuses a well-formed inbound X-Request-ID or generates a new one,
// puts it in the context and echoes it in the response.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(HeaderRequestID)
			if !requestIDPattern.MatchString(id) {
				id = newRequestID()
			}

			w.Header().Set(HeaderRequestID, id)
			next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
		})
	}
}

// RequestIDFrom returns the correlation ID, or "" when the middleware did not run.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

// WithRequestID stores a correlation ID, for background jobs that continue a
// request.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), fallbackID.Add(1))
}
