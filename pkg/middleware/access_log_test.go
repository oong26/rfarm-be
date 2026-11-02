package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.RequestID()(
		middleware.AccessLog(testLogger(&buf))(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("hello"))
			}),
		),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sales?page=1", nil))

	record := logRecord(t, &buf)
	if record["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", record["level"])
	}
	if record["method"] != http.MethodPost {
		t.Errorf("method = %v, want POST", record["method"])
	}
	if record["path"] != "/sales" {
		t.Errorf("path = %v, want /sales", record["path"])
	}
	if record["status"] != float64(http.StatusCreated) {
		t.Errorf("status = %v, want 201", record["status"])
	}
	if record["bytes"] != float64(5) {
		t.Errorf("bytes = %v, want 5", record["bytes"])
	}
	if _, ok := record["latency_ms"]; !ok {
		t.Error("latency_ms is missing")
	}
	if record["request_id"] == "" || record["request_id"] != rec.Header().Get(middleware.HeaderRequestID) {
		t.Errorf("request_id = %v, header = %q", record["request_id"], rec.Header().Get(middleware.HeaderRequestID))
	}
}

func TestAccessLog_Levels(t *testing.T) {
	tests := []struct {
		status    int
		wantLevel string
	}{
		{status: http.StatusOK, wantLevel: "INFO"},
		{status: http.StatusNotFound, wantLevel: "WARN"},
		{status: http.StatusInternalServerError, wantLevel: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			var buf bytes.Buffer
			handler := middleware.AccessLog(testLogger(&buf))(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.status)
				}),
			)

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			if got := logRecord(t, &buf)["level"]; got != tt.wantLevel {
				t.Errorf("level = %v, want %s", got, tt.wantLevel)
			}
		})
	}
}

func TestAccessLog_ImplicitOK(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.AccessLog(testLogger(&buf))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("welcome"))
		}),
	)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	record := logRecord(t, &buf)
	if record["status"] != float64(http.StatusOK) {
		t.Errorf("status = %v, want 200", record["status"])
	}
	if record["bytes"] != float64(7) {
		t.Errorf("bytes = %v, want 7", record["bytes"])
	}
}

func TestAccessLog_IncludesPrincipal(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.RequestID()(
		middleware.AccessLog(testLogger(&buf))(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				middleware.WithPrincipal(r.Context(), middleware.Principal{
					UserID: "user-1",
					FarmID: "farm-1",
				})
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	record := logRecord(t, &buf)
	if record["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1", record["user_id"])
	}
	if record["farm_id"] != "farm-1" {
		t.Errorf("farm_id = %v, want farm-1", record["farm_id"])
	}
}
