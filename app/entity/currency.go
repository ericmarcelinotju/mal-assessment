package entity

import "github.com/shopspring/decimal"

// Currency is an ISO 4217 alphabetic code. Scale is the ISO 4217 exponent: the
// number of decimal places in which amounts of that currency are stored and
// rounded. AED is 2, BHD is 3.
type Currency string

const (
	AED Currency = "AED"
	BHD Currency = "BHD"
)

var scales = map[Currency]int32{
	AED: 2,
	BHD: 3,
}

// Scale returns the number of decimal places for the currency and whether the
// currency is known. An unknown currency is never silently defaulted to 2 --
// defaulting is how a JPY amount ends up a hundred times too small.
func (c Currency) Scale() (int32, bool) {
	s, ok := scales[c]
	return s, ok
}

// MinorUnit is the smallest representable amount in this currency: 0.01 AED,
// 0.001 BHD. Used by the largest-remainder instalment allocator.
func (c Currency) MinorUnit() decimal.Decimal {
	s, ok := c.Scale()
	if !ok {
		return decimal.NewFromInt(1)
	}
	return decimal.New(1, -s)
}

func (c Currency) String() string { return string(c) }
