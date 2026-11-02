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
		wantFields int
	}{
		{
			name:       "bad request",
			err:        response.BadRequest("Body tidak valid."),
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
			wantDetail: "Body tidak valid.",
		},
		{
			name:       "invalid query",
			err:        response.InvalidQuery("Filter tidak valid."),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_query",
			wantDetail: "Filter tidak valid.",
		},
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
			name:       "not found",
			err:        response.NotFound("penjualan"),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantDetail: "Data penjualan tidak ditemukan.",
		},
		{
			name:       "conflict",
			err:        response.Conflict("Data sudah ada."),
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
			wantDetail: "Data sudah ada.",
		},
		{
			name:       "gone",
			err:        response.Gone("Endpoint sudah dihapus."),
			wantStatus: http.StatusGone,
			wantCode:   "gone",
			wantDetail: "Endpoint sudah dihapus.",
		},
		{
			name:       "precondition failed",
			err:        response.PreconditionFailed("Data sudah diubah."),
			wantStatus: http.StatusPreconditionFailed,
			wantCode:   "version_conflict",
			wantDetail: "Data sudah diubah.",
		},
		{
			name:       "precondition required",
			err:        response.PreconditionRequired(),
			wantStatus: http.StatusPreconditionRequired,
			wantCode:   "precondition_required",
			wantDetail: "Header If-Match wajib diisi.",
		},
		{
			name:       "unsupported media",
			err:        response.UnsupportedMedia("Gunakan application/json."),
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   "unsupported_media_type",
			wantDetail: "Gunakan application/json.",
		},
		{
			name: "unprocessable",
			err: response.Unprocessable(
				response.Field{Field: "qty", Code: "min", Message: "Jumlah minimal 1."},
			),
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_failed",
			wantDetail: "",
			wantFields: 1,
		},
		{
			name:       "too many requests",
			err:        response.TooManyRequests("Terlalu banyak permintaan."),
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "too_many_requests",
			wantDetail: "Terlalu banyak permintaan.",
		},
		{
			name:       "internal hides the cause",
			err:        response.Internal(errors.New("database down")),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
			wantDetail: "Terjadi kesalahan pada server.",
		},
		{
			name:       "service unavailable",
			err:        response.ServiceUnavailable("Layanan tidak tersedia."),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "service_unavailable",
			wantDetail: "Layanan tidak tersedia.",
		},
		{
			name:       "gateway timeout",
			err:        response.GatewayTimeout("Permintaan melebihi batas waktu."),
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "gateway_timeout",
			wantDetail: "Permintaan melebihi batas waktu.",
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
			if problem.Title == "" {
				t.Error("problem.title is empty")
			}
			if got, want := problem.Type, "https://rfarm.example/problems/"+tt.wantCode; got != want {
				t.Errorf("problem.type = %q, want %q", got, want)
			}
			if len(problem.Errors) != tt.wantFields {
				t.Errorf("problem.errors length = %d, want %d", len(problem.Errors), tt.wantFields)
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
