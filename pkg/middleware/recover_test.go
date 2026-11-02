package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/middleware"
)

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.RequestID()(
		middleware.Recover(testLogger(&buf))(
			http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic("boom")
			}),
		),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sales", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	problem := decodeProblem(t, rec)
	if problem.Code != "internal" {
		t.Errorf("problem.code = %q, want internal", problem.Code)
	}
	if problem.RequestID == "" || problem.RequestID != rec.Header().Get(middleware.HeaderRequestID) {
		t.Errorf("problem.requestId = %q, header = %q", problem.RequestID, rec.Header().Get(middleware.HeaderRequestID))
	}

	record := logRecord(t, &buf)
	if record["msg"] != "panic recovered" {
		t.Errorf("log msg = %v, want panic recovered", record["msg"])
	}
	if record["panic"] != "boom" {
		t.Errorf("logged panic = %v, want boom", record["panic"])
	}
	if record["request_id"] == "" {
		t.Error("logged request_id is empty")
	}
	if record["stack"] == "" {
		t.Error("logged stack is empty")
	}
	if record["path"] != "/sales" {
		t.Errorf("logged path = %v, want /sales", record["path"])
	}
}

func TestRecover_RePanicsAbortHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := middleware.Recover(testLogger(&buf))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}),
	)

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	if recovered != http.ErrAbortHandler {
		t.Fatalf("recovered = %v, want http.ErrAbortHandler", recovered)
	}
}
