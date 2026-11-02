package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
)

func TestOK(t *testing.T) {
	rec := httptest.NewRecorder()

	response.OK(rec, map[string]any{"id": "sale-1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data["id"] != "sale-1" {
		t.Errorf("data.id = %v, want sale-1", body.Data["id"])
	}
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()

	response.Created(rec, map[string]any{"id": "sale-1", "version": 1})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data["version"] != float64(1) {
		t.Errorf("data.version = %v, want 1", body.Data["version"])
	}
}

func TestNoContent(t *testing.T) {
	rec := httptest.NewRecorder()

	response.NoContent(rec)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func TestList(t *testing.T) {
	rec := httptest.NewRecorder()

	response.List(rec, []string{"a", "b"}, response.NewMeta(2, 25, 137))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data []string      `json:"data"`
		Meta response.Meta `json:"meta"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Data) != 2 || body.Data[0] != "a" || body.Data[1] != "b" {
		t.Errorf("data = %v, want [a b]", body.Data)
	}
	if body.Meta.Page != 2 || body.Meta.PageSize != 25 || body.Meta.Total != 137 {
		t.Errorf("meta = %+v, want {2 25 137}", body.Meta)
	}
}

func TestList_NilSliceIsEmptyArray(t *testing.T) {
	rec := httptest.NewRecorder()
	var items []string

	response.List(rec, items, response.NewMeta(1, 25, 0))

	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := string(body["data"]); got != "[]" {
		t.Errorf("data = %s, want []", got)
	}
}
