package utils

import (
	"time"
)

// DefaultTimezone is Asia/Jakarta, matching the farm locale.
var DefaultTimezone = mustLoadLoc("Asia/Jakarta")

func mustLoadLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("utils: cannot load timezone " + name + ": " + err.Error())
	}
	return loc
}

// Today returns today's date in the given timezone as "YYYY-MM-DD".
// Pass nil to use DefaultTimezone.
func Today(loc *time.Location) string {
	if loc == nil {
		loc = DefaultTimezone
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// CurrentMonth returns the current month in the given timezone as "YYYY-MM".
// Pass nil to use DefaultTimezone.
func CurrentMonth(loc *time.Location) string {
	if loc == nil {
		loc = DefaultTimezone
	}
	return time.Now().In(loc).Format("2006-01")
}

// CurrentYear returns the current year in the given timezone.
// Pass nil to use DefaultTimezone.
func CurrentYear(loc *time.Location) int {
	if loc == nil {
		loc = DefaultTimezone
	}
	return time.Now().In(loc).Year()
}

// ParseDate parses a "YYYY-MM-DD" string. Returns zero time and error on
// invalid input.
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// FormatDate formats a time.Time to "YYYY-MM-DD".
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// ParseMonth parses a "YYYY-MM" string. Returns zero time and error on
// invalid input.
func ParseMonth(s string) (time.Time, error) {
	return time.Parse("2006-01", s)
}

// FormatMonth formats a time.Time to "YYYY-MM".
func FormatMonth(t time.Time) string {
	return t.Format("2006-01")
}

// MonthRange returns the inclusive first and last dates for a "YYYY-MM" month.
func MonthRange(month string) (first, last string) {
	t, _ := time.Parse("2006-01", month)
	first = t.Format("2006-01-02")
	last = t.AddDate(0, 1, -1).Format("2006-01-02")
	return first, last
}

// BeginningOfMonth returns the first day of t's month.
func BeginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth returns the last day of t's month (inclusive).
func EndOfMonth(t time.Time) time.Time {
	return BeginningOfMonth(t).AddDate(0, 1, -1)
}

// NowUTC returns the current UTC time, for audit timestamps.
func NowUTC() time.Time {
	return time.Now().UTC()
}
