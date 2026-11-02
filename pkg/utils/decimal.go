package utils

import (
	"github.com/shopspring/decimal"
)

// DecimalZero is a convenience constant.
var DecimalZero = decimal.Zero

// NewDecimal creates a decimal from a float64. Prefer string-based
// constructors for money; this is for converting validated numeric input.
func NewDecimal(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

// NewDecimalFromString creates a decimal from a numeric string.
// Returns an error when s is not a valid number.
func NewDecimalFromString(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}

// RoundMoney rounds d to 2 decimal places using banker's rounding.
func RoundMoney(d decimal.Decimal) decimal.Decimal {
	return d.Round(2)
}

// MaxDecimal returns the larger of a and b.
func MaxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThanOrEqual(b) {
		return a
	}
	return b
}

// MinDecimal returns the smaller of a and b.
func MinDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThanOrEqual(b) {
		return a
	}
	return b
}

// SafeDivide returns a/b, or zero when b is zero.
func SafeDivide(a, b decimal.Decimal) decimal.Decimal {
	if b.IsZero() {
		return decimal.Zero
	}
	return a.Div(b)
}

// ClampNonNegative returns max(d, 0).
func ClampNonNegative(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}
