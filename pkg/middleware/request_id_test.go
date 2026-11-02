package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func TestRequestID(t *testing.T) {
	serve := func(req *http.Request) (*httptest.ResponseRecorder, string) {
		var fromContext string
		handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fromContext = middleware.RequestIDFrom(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec, fromContext
	}

	t.Run("generates an id", func(t *testing.T) {
		rec, fromContext := serve(httptest.NewRequest(http.MethodGet, "/", nil))

		if fromContext == "" {
			t.Fatal("request ID missing from context")
		}
		if got := rec.Header().Get(middleware.HeaderRequestID); got != fromContext {
			t.Errorf("response header = %q, want %q", got, fromContext)
		}
	})

	t.Run("reuses a well-formed id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(middleware.HeaderRequestID, "client-request_123")

		rec, fromContext := serve(req)

		if fromContext != "client-request_123" {
			t.Errorf("context id = %q, want client-request_123", fromContext)
		}
		if got := rec.Header().Get(middleware.HeaderRequestID); got != "client-request_123" {
			t.Errorf("response header = %q, want client-request_123", got)
		}
	})

	t.Run("replaces a malformed id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(middleware.HeaderRequestID, "bad id with spaces")

		rec, fromContext := serve(req)

		if fromContext == "" || fromContext == "bad id with spaces" {
			t.Errorf("context id = %q, want a generated id", fromContext)
		}
		if got := rec.Header().Get(middleware.HeaderRequestID); got != fromContext {
			t.Errorf("response header = %q, want %q", got, fromContext)
		}
	})

	t.Run("ids are unique", func(t *testing.T) {
		_, first := serve(httptest.NewRequest(http.MethodGet, "/", nil))
		_, second := serve(httptest.NewRequest(http.MethodGet, "/", nil))

		if first == second {
			t.Errorf("two requests received the same id %q", first)
		}
	})
}

func TestWithRequestID(t *testing.T) {
	ctx := middleware.WithRequestID(context.Background(), "job-1")

	if got := middleware.RequestIDFrom(ctx); got != "job-1" {
		t.Errorf("RequestIDFrom() = %q, want job-1", got)
	}
}
