package utils_test

import (
	"testing"
	"time"

	"github.com/oong26/rfarm-be/pkg/utils"
	"github.com/shopspring/decimal"
)

// ---------- Clock / Date ----------

func TestToday(t *testing.T) {
	today := utils.Today(nil)
	if _, err := time.Parse("2006-01-02", today); err != nil {
		t.Fatalf("invalid format: %s", today)
	}
}

func TestCurrentMonth(t *testing.T) {
	m := utils.CurrentMonth(nil)
	if _, err := time.Parse("2006-01", m); err != nil {
		t.Fatalf("invalid format: %s", m)
	}
}

func TestCurrentYear(t *testing.T) {
	y := utils.CurrentYear(nil)
	if y < 2024 || y > 2100 {
		t.Fatalf("unreasonable year: %d", y)
	}
}

func TestParseDate_RoundTrip(t *testing.T) {
	tm, err := utils.ParseDate("2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	if utils.FormatDate(tm) != "2026-10-31" {
		t.Fatalf("formatted = %s, want 2026-10-31", utils.FormatDate(tm))
	}
}

func TestParseMonth_RoundTrip(t *testing.T) {
	tm, err := utils.ParseMonth("2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if utils.FormatMonth(tm) != "2026-10" {
		t.Fatalf("formatted = %s, want 2026-10", utils.FormatMonth(tm))
	}
}

func TestMonthRange(t *testing.T) {
	first, last := utils.MonthRange("2026-10")
	if first != "2026-10-01" || last != "2026-10-31" {
		t.Fatalf("got %s..%s, want 2026-10-01..2026-10-31", first, last)
	}
}

func TestBeginningOfMonth(t *testing.T) {
	tm := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	b := utils.BeginningOfMonth(tm)
	if b.Day() != 1 || b.Month() != 10 || b.Year() != 2026 {
		t.Fatalf("got %+v", b)
	}
}

func TestEndOfMonth(t *testing.T) {
	tm := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	e := utils.EndOfMonth(tm)
	if e.Day() != 31 || e.Month() != 10 || e.Year() != 2026 {
		t.Fatalf("got %+v", e)
	}
}

func TestEndOfMonth_February(t *testing.T) {
	tm := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	e := utils.EndOfMonth(tm)
	if e.Day() != 28 {
		t.Fatalf("got day %d, want 28", e.Day())
	}
}

// ---------- Decimal ----------

func TestNewDecimal(t *testing.T) {
	d := utils.NewDecimal(12.34)
	if !d.Equal(decimal.NewFromFloat(12.34)) {
		t.Fatalf("got %s", d)
	}
}

func TestNewDecimalFromString(t *testing.T) {
	d, err := utils.NewDecimalFromString("44000.50")
	if err != nil {
		t.Fatal(err)
	}
	if d.StringFixed(2) != "44000.50" {
		t.Fatalf("got %s", d.StringFixed(2))
	}
}

func TestRoundMoney(t *testing.T) {
	d := decimal.RequireFromString("12.3456")
	rounded := utils.RoundMoney(d)
	if rounded.StringFixed(2) != "12.35" {
		t.Fatalf("got %s, want 12.35", rounded.StringFixed(2))
	}
}

func TestMaxDecimal(t *testing.T) {
	a := decimal.RequireFromString("10")
	b := decimal.RequireFromString("20")
	if !utils.MaxDecimal(a, b).Equal(b) {
		t.Fatalf("max(10, 20) != 20")
	}
}

func TestMinDecimal(t *testing.T) {
	a := decimal.RequireFromString("10")
	b := decimal.RequireFromString("20")
	if !utils.MinDecimal(a, b).Equal(a) {
		t.Fatalf("min(10, 20) != 10")
	}
}

func TestSafeDivide(t *testing.T) {
	a := decimal.RequireFromString("10")
	b := decimal.Zero
	if !utils.SafeDivide(a, b).IsZero() {
		t.Fatalf("divide by zero should be zero")
	}

	c := decimal.RequireFromString("2")
	if !utils.SafeDivide(a, c).Equal(decimal.RequireFromString("5")) {
		t.Fatalf("10/2 != 5")
	}
}

func TestClampNonNegative(t *testing.T) {
	neg := decimal.RequireFromString("-5")
	if !utils.ClampNonNegative(neg).IsZero() {
		t.Fatalf("clamped negative should be zero")
	}

	pos := decimal.RequireFromString("5")
	if !utils.ClampNonNegative(pos).Equal(pos) {
		t.Fatalf("positive should remain unchanged")
	}
}
