package response_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
)

func TestFail(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "unauthorized",
			err:        response.Unauthorized("Sesi tidak valid."),
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
			wantDetail: "Sesi tidak valid.",
		},
		{
			name:       "forbidden",
			err:        response.Forbidden("Tidak diizinkan."),
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
			wantDetail: "Tidak diizinkan.",
		},
		{
			name:       "internal hides the cause",
			err:        response.Internal(errors.New("database down")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
			wantDetail: "Terjadi kesalahan pada server.",
		},
		{
			name:       "unknown error becomes internal",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
			wantDetail: "Terjadi kesalahan pada server.",
		},
		{
			name:       "wrapped app error keeps its status",
			err:        fmt.Errorf("handler: %w", response.Unauthorized("expired")),
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
			wantDetail: "expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rec.Header().Set("X-Request-ID", "req-1")
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			response.Fail(rec, req, tt.err)

			result := rec.Result()
			if result.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", result.StatusCode, tt.wantStatus)
			}
			if got := result.Header.Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", got)
			}

			var problem response.Problem
			if err := json.NewDecoder(result.Body).Decode(&problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}

			if problem.Status != tt.wantStatus {
				t.Errorf("problem.status = %d, want %d", problem.Status, tt.wantStatus)
			}
			if problem.Code != tt.wantCode {
				t.Errorf("problem.code = %q, want %q", problem.Code, tt.wantCode)
			}
			if problem.Detail != tt.wantDetail {
				t.Errorf("problem.detail = %q, want %q", problem.Detail, tt.wantDetail)
			}
			if problem.RequestID != "req-1" {
				t.Errorf("problem.requestId = %q, want req-1", problem.RequestID)
			}
			if got, want := problem.Type, "https://rfarm.example/problems/"+tt.wantCode; got != want {
				t.Errorf("problem.type = %q, want %q", got, want)
			}
		})
	}
}

func TestFail_HidesCause(t *testing.T) {
	rec := httptest.NewRecorder()

	response.Fail(rec, httptest.NewRequest(http.MethodGet, "/", nil),
		errors.New("password authentication failed for user"))

	if strings.Contains(rec.Body.String(), "password authentication failed") {
		t.Errorf("response leaked the internal cause: %s", rec.Body.String())
	}
}
