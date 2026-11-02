package utils_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oong26/rfarm-be/pkg/response"
	"github.com/oong26/rfarm-be/pkg/utils"
)

// ---------- ParsePage ----------

func TestParsePage_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	p, err := utils.ParsePage(r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Number != 1 || p.Size != 25 {
		t.Fatalf("page = %+v, want {1, 25}", p)
	}
	if p.Offset() != 0 {
		t.Fatalf("offset = %d, want 0", p.Offset())
	}
}

func TestParsePage_Custom(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=3&pageSize=50", nil)
	p, err := utils.ParsePage(r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Number != 3 || p.Size != 50 {
		t.Fatalf("page = %+v, want {3, 50}", p)
	}
	if p.Offset() != 100 {
		t.Fatalf("offset = %d, want 100", p.Offset())
	}
}

func TestParsePage_ExceedsMax(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?pageSize=500", nil)
	_, err := utils.ParsePage(r)
	if err == nil {
		t.Fatal("expected error for pageSize > 200")
	}
}

func TestParsePage_NegativePage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=-1", nil)
	p, err := utils.ParsePage(r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Number != 1 {
		t.Fatalf("page = %d, want 1 (clamped)", p.Number)
	}
}

// ---------- ParseSort ----------

func TestParseSort_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s, err := utils.ParseSort(r, []string{"date", "buyer"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Field != "" {
		t.Fatalf("field = %q, want empty", s.Field)
	}
}

func TestParseSort_Ascending(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?sort=buyer", nil)
	s, err := utils.ParseSort(r, []string{"date", "buyer"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Field != "buyer" || s.Desc {
		t.Fatalf("sort = %+v, want {buyer, false}", s)
	}
}

func TestParseSort_Descending(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?sort=-date", nil)
	s, err := utils.ParseSort(r, []string{"date", "buyer"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Field != "date" || !s.Desc {
		t.Fatalf("sort = %+v, want {date, true}", s)
	}
}

func TestParseSort_NotInAllowlist(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?sort=unknown", nil)
	_, err := utils.ParseSort(r, []string{"date", "buyer"})
	if err == nil {
		t.Fatal("expected error for unknown sort field")
	}
}

// ---------- ParseDateRange ----------

func TestParseDateRange_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	dr, err := utils.ParseDateRange(r)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Month != "" || dr.From != "" || dr.To != "" {
		t.Fatalf("expected empty, got %+v", dr)
	}
}

func TestParseDateRange_Month(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?month=2026-10", nil)
	dr, err := utils.ParseDateRange(r)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Month != "2026-10" {
		t.Fatalf("month = %q", dr.Month)
	}
}

func TestParseDateRange_FromTo(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-10-01&to=2026-10-31", nil)
	dr, err := utils.ParseDateRange(r)
	if err != nil {
		t.Fatal(err)
	}
	if dr.From != "2026-10-01" || dr.To != "2026-10-31" {
		t.Fatalf("range = %+v", dr)
	}
}

func TestParseDateRange_BothMonthAndFrom(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?month=2026-10&from=2026-10-01", nil)
	_, err := utils.ParseDateRange(r)
	if err == nil {
		t.Fatal("expected error for month + from")
	}
}

func TestParseDateRange_FromAfterTo(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-10-31&to=2026-10-01", nil)
	_, err := utils.ParseDateRange(r)
	if err == nil {
		t.Fatal("expected error for from > to")
	}
}

func TestParseDateRange_InvalidMonth(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?month=2026-13", nil)
	_, err := utils.ParseDateRange(r)
	if err == nil {
		t.Fatal("expected error for invalid month")
	}
}

func TestParseDateRange_PartialFromTo(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-10-01", nil)
	_, err := utils.ParseDateRange(r)
	if err == nil {
		t.Fatal("expected error when only from is given")
	}
}

// ---------- ResolveMonthToRange ----------

func TestResolveMonthToRange(t *testing.T) {
	from, to := utils.ResolveMonthToRange("2026-02")
	if from != "2026-02-01" || to != "2026-02-28" {
		t.Fatalf("got %s..%s", from, to)
	}
}

func TestResolveMonthToRange_LeapYear(t *testing.T) {
	from, to := utils.ResolveMonthToRange("2024-02")
	if from != "2024-02-01" || to != "2024-02-29" {
		t.Fatalf("got %s..%s", from, to)
	}
}

// ---------- SearchQuery ----------

func TestSearchQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?q=bondowoso", nil)
	q := utils.SearchQuery(r)
	if q != "bondowoso" {
		t.Fatalf("got %q", q)
	}
}

// ---------- ParseEnumQuery ----------

func TestParseEnumQuery_Valid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?status=given", nil)
	v, err := utils.ParseEnumQuery(r, "status", []string{"given", "stock"})
	if err != nil {
		t.Fatal(err)
	}
	if v != "given" {
		t.Fatalf("got %q", v)
	}
}

func TestParseEnumQuery_Invalid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?status=bad", nil)
	_, err := utils.ParseEnumQuery(r, "status", []string{"given", "stock"})
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *response.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want *AppError, got %T", err)
	}
}

func TestParseEnumQuery_Absent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	v, err := utils.ParseEnumQuery(r, "status", []string{"given", "stock"})
	if err != nil {
		t.Fatal(err)
	}
	if v != "" {
		t.Fatalf("got %q, want empty", v)
	}
}
