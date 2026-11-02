package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func serveFarmScope(principal *middleware.Principal, req *http.Request) (rec *httptest.ResponseRecorder, nextCalled bool, farmID string) {
	injectPrincipal := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal != nil {
				r = r.WithContext(middleware.WithPrincipal(r.Context(), *principal))
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := middleware.RequestID()(
		injectPrincipal(
			middleware.FarmScope()(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					nextCalled = true
					farmID = middleware.FarmIDFrom(r.Context())
					w.WriteHeader(http.StatusNoContent)
				}),
			),
		),
	)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, nextCalled, farmID
}

func TestFarmScope(t *testing.T) {
	t.Run("allows an authenticated farm", func(t *testing.T) {
		rec, nextCalled, farmID := serveFarmScope(
			&middleware.Principal{UserID: "user-1", FarmID: "farm-1"},
			httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil),
		)

		if !nextCalled {
			t.Error("next handler did not run")
		}
		if farmID != "farm-1" {
			t.Errorf("FarmIDFrom() = %q, want farm-1", farmID)
		}
		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("rejects a missing principal", func(t *testing.T) {
		rec, nextCalled, _ := serveFarmScope(nil,
			httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil))

		if nextCalled {
			t.Error("next handler ran without a principal")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if problem := decodeProblem(t, rec); problem.Code != "unauthorized" {
			t.Errorf("problem.code = %q, want unauthorized", problem.Code)
		}
	})

	t.Run("rejects an empty farm", func(t *testing.T) {
		rec, nextCalled, _ := serveFarmScope(
			&middleware.Principal{UserID: "user-1"},
			httptest.NewRequest(http.MethodGet, "/api/v1/sales", nil),
		)

		if nextCalled {
			t.Error("next handler ran without a farm")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("ignores a client-supplied farm", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sales?farmId=farm-evil", nil)
		req.Header.Set("X-Farm-ID", "farm-evil")

		rec, nextCalled, _ := serveFarmScope(nil, req)

		if nextCalled {
			t.Error("next handler ran from a client-supplied farm")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
