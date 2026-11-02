package utils

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oong26/rfarm-be/pkg/response"
)

// PageParams holds the paginated list parameters after defaults and clamping.
type PageParams struct {
	Number int // 1-based, >= 1
	Size   int // 1..200
}

const (
	DefaultPageSize = 25
	MaxPageSize     = 200
)

// ParsePage reads page and pageSize from the query string with defaults and
// clamping. It returns an InvalidQuery error when pageSize > MaxPageSize.
func ParsePage(r *http.Request) (PageParams, error) {
	page, err := QueryInt(r, "page", 1)
	if err != nil {
		return PageParams{}, err
	}
	if page < 1 {
		page = 1
	}

	pageSize, err := QueryInt(r, "pageSize", DefaultPageSize)
	if err != nil {
		return PageParams{}, err
	}

	if pageSize > MaxPageSize {
		return PageParams{}, response.InvalidQuery(
			fmt.Sprintf("pageSize tidak boleh melebihi %d.", MaxPageSize),
		)
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}

	return PageParams{Number: page, Size: pageSize}, nil
}

// Offset returns the SQL LIMIT/OFFSET offset for this page.
func (p PageParams) Offset() int {
	return (p.Number - 1) * p.Size
}

// SortParams holds the parsed sort parameter.
type SortParams struct {
	Field string // without leading '-'
	Desc  bool
}

// ParseSort reads the sort query parameter against an allowlist. The wire
// format is a single field name, optionally prefixed with '-' for descending.
// Returns InvalidQuery when the field is not in the allowlist.
func ParseSort(r *http.Request, allowlist []string) (SortParams, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("sort"))
	if raw == "" {
		return SortParams{}, nil
	}

	desc := false
	field := raw
	if strings.HasPrefix(raw, "-") {
		desc = true
		field = raw[1:]
	}

	ok := false
	for _, a := range allowlist {
		if a == field {
			ok = true
			break
		}
	}
	if !ok {
		return SortParams{}, response.InvalidQuery(
			fmt.Sprintf("Field sort '%s' tidak diizinkan. Pilihan: %s",
				field, strings.Join(allowlist, ", ")),
		)
	}
	return SortParams{Field: field, Desc: desc}, nil
}

// DateRange holds the resolved inclusive date range for a list query.
// Exactly one of Month or From/To is set; both cannot be present together.
type DateRange struct {
	Month string // "YYYY-MM" if month was given
	From  string // "YYYY-MM-DD"
	To    string // "YYYY-MM-DD"
}

// ParseDateRange reads month, from, and to query parameters.
// Returns an InvalidQuery error when both month and from/to are provided,
// when from > to, or when a date fails to parse.
func ParseDateRange(r *http.Request) (DateRange, error) {
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))

	if month != "" && (from != "" || to != "") {
		return DateRange{}, response.InvalidQuery(
			"Parameter 'month' dan 'from'/'to' tidak boleh dipakai bersamaan.",
		)
	}

	if month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			return DateRange{}, response.InvalidQuery(
				"Parameter 'month' harus berformat YYYY-MM.",
			)
		}
		return DateRange{Month: month}, nil
	}

	if from != "" || to != "" {
		if from == "" || to == "" {
			return DateRange{}, response.InvalidQuery(
				"Parameter 'from' dan 'to' harus diisi keduanya.",
			)
		}
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return DateRange{}, response.InvalidQuery(
				"Parameter 'from' harus berformat YYYY-MM-DD.",
			)
		}
		if _, err := time.Parse("2006-01-02", to); err != nil {
			return DateRange{}, response.InvalidQuery(
				"Parameter 'to' harus berformat YYYY-MM-DD.",
			)
		}
		if from > to {
			return DateRange{}, response.InvalidQuery(
				"Parameter 'from' tidak boleh setelah 'to'.",
			)
		}
		return DateRange{From: from, To: to}, nil
	}

	return DateRange{}, nil
}

// ResolveMonthToRange expands a "YYYY-MM" month string into an inclusive
// date range (first day of month to first day of next month minus one day).
func ResolveMonthToRange(month string) (from, to string) {
	t, _ := time.Parse("2006-01", month)
	from = t.Format("2006-01-02")
	to = t.AddDate(0, 1, -1).Format("2006-01-02")
	return from, to
}

// SearchQuery reads the free-text search parameter q. Returns empty string
// when absent or blank.
func SearchQuery(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("q"))
}

// ParseIntQuery is a generic int query parser that returns an error on
// invalid input (distinct from missing). Use this for custom int filters.
func ParseIntQuery(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, response.InvalidQuery(
			fmt.Sprintf("Parameter '%s' wajib diisi.", key),
		)
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, response.InvalidQuery(
			fmt.Sprintf("Parameter '%s' harus berupa bilangan bulat.", key),
		)
	}
	return v, nil
}

// ParseOptionalIntQuery reads an optional int query param. Returns (0, nil)
// when absent or blank. Returns error only on non-empty invalid input.
func ParseOptionalIntQuery(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, response.InvalidQuery(
			fmt.Sprintf("Parameter '%s' harus berupa bilangan bulat.", key),
		)
	}
	return v, nil
}

// ParseEnumQuery reads an optional enum query param against an allowlist.
// Returns empty string when absent/blank. Returns InvalidQuery when the
// value is not in the allowlist.
func ParseEnumQuery(r *http.Request, key string, allowlist []string) (string, error) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return "", nil
	}
	for _, a := range allowlist {
		if a == v {
			return v, nil
		}
	}
	return "", response.InvalidQuery(
		fmt.Sprintf("Parameter '%s' harus salah satu dari: %s",
			key, strings.Join(allowlist, ", ")),
	)
}