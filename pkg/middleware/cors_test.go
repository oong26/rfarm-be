package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func serveCORS(allowed []string, req *http.Request) (*httptest.ResponseRecorder, bool) {
	nextCalled := false
	handler := middleware.CORS(allowed)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, nextCalled
}

func TestCORS_PreflightAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/sales", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	rec, nextCalled := serveCORS([]string{"http://localhost:5173"}, req)

	if nextCalled {
		t.Error("next handler ran for a preflight request")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow-origin = %q, want http://localhost:5173", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Errorf("allow-methods = %q, want POST", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "If-Match") {
		t.Errorf("allow-headers = %q, want If-Match", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("max-age = %q, want 600", got)
	}

	vary := strings.Join(rec.Header().Values("Vary"), ", ")
	if !strings.Contains(vary, "Origin") || !strings.Contains(vary, "Access-Control-Request-Method") {
		t.Errorf("Vary = %q, want Origin and Access-Control-Request-Method", vary)
	}
}

func TestCORS_PreflightDenied(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/sales", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	rec, nextCalled := serveCORS([]string{"http://localhost:5173"}, req)

	if nextCalled {
		t.Error("next handler ran for a denied preflight request")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want empty", got)
	}
	if problem := decodeProblem(t, rec); problem.Code != "forbidden" {
		t.Errorf("problem.code = %q, want forbidden", problem.Code)
	}
}

func TestCORS_SimpleRequest(t *testing.T) {
	t.Run("allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil)
		req.Header.Set("Origin", "http://localhost:5173")

		rec, nextCalled := serveCORS([]string{"http://localhost:5173"}, req)

		if !nextCalled {
			t.Error("next handler did not run")
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("allow-origin = %q, want http://localhost:5173", got)
		}
		if got := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "ETag") {
			t.Errorf("expose-headers = %q, want ETag", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
			t.Errorf("allow-methods = %q, want empty on a simple request", got)
		}
	})

	t.Run("denied origin still reaches the handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil)
		req.Header.Set("Origin", "https://evil.example")

		rec, nextCalled := serveCORS([]string{"http://localhost:5173"}, req)

		if !nextCalled {
			t.Error("next handler did not run")
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want empty", got)
		}
	})

	t.Run("no origin", func(t *testing.T) {
		rec, nextCalled := serveCORS([]string{"http://localhost:5173"},
			httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil))

		if !nextCalled {
			t.Error("next handler did not run")
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("allow-origin = %q, want empty", got)
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil)
		req.Header.Set("Origin", "http://localhost:3000")

		rec, nextCalled := serveCORS([]string{"*"}, req)

		if !nextCalled {
			t.Error("next handler did not run")
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("allow-origin = %q, want *", got)
		}
	})
}
