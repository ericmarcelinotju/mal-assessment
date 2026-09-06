package entity

import (
	"strings"

	"github.com/shopspring/decimal"

	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Money is an exact decimal amount tagged with its currency, built on
// shopspring/decimal so the arithmetic is exact rather than binary-floating.
//
// The invariant this type adds on top of decimal.Decimal is quantisation:
// every Money is rounded to its own currency's scale at construction, so a
// value carrying sub-minor-unit dust cannot exist. That matters because
// decimal.Decimal is arbitrary precision -- 10/3 is happily representable to 16
// places, and a ledger that lets such a value survive will fail to reconcile
// later. Rounding happens once, here, at the boundary.
//
// Money is a value type. Every operation returns a new Money; nothing mutates.
type Money struct {
	amount decimal.Decimal
	ccy    Currency
}

// NewMoney quantises d to the currency's scale, rounding half away from zero.
func NewMoney(d decimal.Decimal, ccy Currency) Money {
	scale, ok := ccy.Scale()
	if !ok {
		panic(apperror.New(apperror.ErrCurrencyMismatch, "unknown currency "+string(ccy)))
	}
	return Money{amount: d.Round(scale), ccy: ccy}
}

// Zero is the additive identity for a currency.
func Zero(ccy Currency) Money { return NewMoney(decimal.Zero, ccy) }

// ParseMoney builds an amount from a decimal string such as "1200.00" or
// "10.000". It rejects any input carrying more decimal places than the currency
// allows, rather than rounding it away: silently truncating input is how a
// ledger loses money it was told about. Contrast NewMoney, which rounds
// deliberately because its input is a computed value, not a stated one.
func ParseMoney(s string, ccy Currency) (Money, error) {
	scale, ok := ccy.Scale()
	if !ok {
		return Money{}, apperror.New(apperror.ErrCurrencyMismatch, "unknown currency "+string(ccy))
	}
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")

	d, err := decimal.NewFromString(s)
	if err != nil {
		return Money{}, apperror.New(apperror.ErrInvalidParameter, "malformed amount "+s, err)
	}
	if -d.Exponent() > scale {
		return Money{}, apperror.New(apperror.ErrInvalidParameter,
			"amount "+s+" has more precision than "+string(ccy)+" allows")
	}
	return Money{amount: d.Round(scale), ccy: ccy}, nil
}

// MustParseMoney is for the canonical event stream and tests, where the inputs
// are literals fixed at compile time.
func MustParseMoney(s string, ccy Currency) Money {
	m, err := ParseMoney(s, ccy)
	if err != nil {
		panic(err)
	}
	return m
}

func (m Money) Decimal() decimal.Decimal { return m.amount }
func (m Money) Currency() Currency       { return m.ccy }
func (m Money) IsZero() bool             { return m.amount.IsZero() }
func (m Money) IsNegative() bool         { return m.amount.IsNegative() }
func (m Money) IsPositive() bool         { return m.amount.IsPositive() }

// Add returns m+other. Adding across currencies is a programming error, not a
// runtime condition to be handled: the ledger never holds mixed-currency
// entries on one account, so this panics rather than returning an error that
// every call site would have to ignore.
func (m Money) Add(other Money) Money {
	m.assertSameCurrency(other)
	return Money{amount: m.amount.Add(other.amount), ccy: m.ccy}
}

func (m Money) Sub(other Money) Money {
	m.assertSameCurrency(other)
	return Money{amount: m.amount.Sub(other.amount), ccy: m.ccy}
}

func (m Money) Neg() Money { return Money{amount: m.amount.Neg(), ccy: m.ccy} }

func (m Money) Cmp(other Money) int {
	m.assertSameCurrency(other)
	return m.amount.Cmp(other.amount)
}

func (m Money) Equal(other Money) bool {
	return m.ccy == other.ccy && m.amount.Equal(other.amount)
}

func (m Money) assertSameCurrency(other Money) {
	if m.ccy != other.ccy {
		panic(apperror.New(apperror.ErrCurrencyMismatch,
			"cannot combine "+string(m.ccy)+" and "+string(other.ccy)))
	}
}

// MulRatioHalfUp returns round(m * num/den) at the currency's own scale,
// rounding half away from zero (decimal.Round's mode).
//
// The rate is kept as an exact rational rather than a parsed literal so that
// "0.04% per day" never passes through a float64. Add and Sub above are exact,
// so this is the only rounding in the entire interest calculation.
func (m Money) MulRatioHalfUp(num, den int64) Money {
	if den == 0 {
		panic(apperror.New(apperror.ErrInternalValidation, "division by zero in MulRatioHalfUp"))
	}
	raw := m.amount.Mul(decimal.NewFromInt(num)).Div(decimal.NewFromInt(den))
	return NewMoney(raw, m.ccy)
}

// String renders the amount at the currency's own scale, e.g. "-370.00" or
// "10.000". The scale is never inferred from the value, so a whole 10 BHD
// prints as "10.000" and not "10".
func (m Money) String() string {
	scale, ok := m.ccy.Scale()
	if !ok {
		return m.amount.String()
	}
	return m.amount.StringFixed(scale)
}

// Display renders the amount with its currency code, for report output.
func (m Money) Display() string { return m.String() + " " + string(m.ccy) }
