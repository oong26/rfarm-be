package utils_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
	"github.com/oong26/rfarm-be/pkg/utils"
)

// ---------- ParseAndValidate ----------

type sampleRequest struct {
	Date     string `json:"date"     validate:"required,datetime=2006-01-02"`
	Birds    int    `json:"birds"    validate:"required,min=0,max=10000"`
	Buyer    string `json:"buyer"    validate:"omitempty,max=120"`
	Payment  string `json:"payment"  validate:"omitempty,oneof=cash transfer qris credit"`
}

func TestParseAndValidate_OK(t *testing.T) {
	body := `{"date":"2026-10-31","birds":30,"buyer":"Pak Slamet","payment":"cash"}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	var req sampleRequest
	if err := utils.ParseAndValidate(r, &req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Birds != 30 {
		t.Fatalf("birds = %d, want 30", req.Birds)
	}
}

func TestParseAndValidate_MalformedJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{bad`))
	var req sampleRequest
	err := utils.ParseAndValidate(r, &req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
	if appErr.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", appErr.Status)
	}
}

func TestParseAndValidate_ValidationErrors(t *testing.T) {
	body := `{"date":"","birds":-1,"payment":"barter"}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	var req sampleRequest
	err := utils.ParseAndValidate(r, &req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
	if appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", appErr.Status)
	}
	if len(appErr.Fields) == 0 {
		t.Fatal("expected field errors")
	}
}

func TestParseAndValidate_UnknownField(t *testing.T) {
	body := `{"date":"2026-10-31","birds":30,"unknown_field":true}`
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	var req sampleRequest
	err := utils.ParseAndValidate(r, &req)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
	if appErr.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", appErr.Status)
	}
}

// ---------- Validate (standalone) ----------

func TestValidate_OK(t *testing.T) {
	req := sampleRequest{Date: "2026-10-31", Birds: 30, Payment: "cash"}
	if err := utils.Validate(&req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	req := sampleRequest{Date: "", Birds: -1}
	err := utils.Validate(&req)
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
	if appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", appErr.Status)
	}
}

// ---------- QueryInt / QueryString ----------

func TestQueryInt_Default(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	v, err := utils.QueryInt(r, "page", 1)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("got %d, want 1", v)
	}
}

func TestQueryInt_Present(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=5", nil)
	v, err := utils.QueryInt(r, "page", 1)
	if err != nil {
		t.Fatal(err)
	}
	if v != 5 {
		t.Fatalf("got %d, want 5", v)
	}
}

func TestQueryInt_Invalid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=abc", nil)
	_, err := utils.QueryInt(r, "page", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryString_Default(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	v := utils.QueryString(r, "q", "")
	if v != "" {
		t.Fatalf("got %q, want empty", v)
	}
}

func TestQueryString_Present(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?q=bondowoso", nil)
	v := utils.QueryString(r, "q", "")
	if v != "bondowoso" {
		t.Fatalf("got %q, want bondowoso", v)
	}
}
