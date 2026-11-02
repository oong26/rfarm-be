package utils_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
	"github.com/oong26/rfarm-be/pkg/utils"
)

// ---------- ParseIfMatch ----------

func TestParseIfMatch_Valid(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("If-Match", "4")
	v, err := utils.ParseIfMatch(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 4 {
		t.Fatalf("got %d, want 4", v)
	}
}

func TestParseIfMatch_Quoted(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("If-Match", `"4"`)
	v, err := utils.ParseIfMatch(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 4 {
		t.Fatalf("got %d, want 4", v)
	}
}

func TestParseIfMatch_Absent(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	v, err := utils.ParseIfMatch(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
}

func TestParseIfMatch_Wildcard(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("If-Match", "*")
	_, err := utils.ParseIfMatch(r)
	if err == nil {
		t.Fatal("expected error for wildcard '*'")
	}
}

func TestParseIfMatch_Negative(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("If-Match", "-1")
	_, err := utils.ParseIfMatch(r)
	if err == nil {
		t.Fatal("expected error for negative version")
	}
}

// ---------- RequireIfMatch ----------

func TestRequireIfMatch_Present(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	r.Header.Set("If-Match", "3")
	v, err := utils.RequireIfMatch(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 3 {
		t.Fatalf("got %d, want 3", v)
	}
}

func TestRequireIfMatch_Absent(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	_, err := utils.RequireIfMatch(r)
	if err == nil {
		t.Fatal("expected error when If-Match is absent")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
	if appErr.Status != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428", appErr.Status)
	}
}
