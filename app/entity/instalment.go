package entity

import (
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// SplitInstalments divides an amount into n parts that sum exactly to the
// original, using largest-remainder allocation.
//
// This is where acceptance criterion 7 fails. BHD 10.000 into three "equal"
// instalments is not satisfiable: BHD stores three decimals, and no 3dp value x
// has 3x = 10.000. The brief asks for two things that cannot both hold --
// equality between the parts, and a total of 10.000 -- so one has to give.
//
//	3 x 3.334 = 10.002   invents 0.002 that nobody credited
//	3 x 3.333 =  9.999   loses 0.001 that somebody did
//	3.334 + 3.333 + 3.333 = 10.000  exact, parts differ by one minor unit
//
// Conservation of value wins over equality, because equality is a description
// of how the credit is delivered while 10.000 is the credit. The allocation is
// the minimum-variance one: every part is the floor, and the remainder is
// distributed one minor unit at a time, so no two parts differ by more than a
// single minor unit.
//
// The residual goes to the earliest instalments. That is a convention, not a
// derivation -- see AMBIGUITIES.md -- but it is a fixed one, so the split is
// deterministic and reproducible rather than dependent on map ordering.
func SplitInstalments(total Money, n int) ([]Money, error) {
	if n <= 0 {
		return nil, apperror.New(apperror.ErrInvalidParameter,
			"instalment count must be positive, got "+strconv.Itoa(n))
	}
	if n == 1 {
		return []Money{total}, nil
	}

	ccy := total.Currency()
	scale, ok := ccy.Scale()
	if !ok {
		return nil, apperror.New(apperror.ErrCurrencyMismatch, "unknown currency "+ccy.String())
	}

	// Work in minor units so the arithmetic is integral and the remainder is
	// exactly the number of minor units left to hand out.
	unit := decimal.New(1, -scale)
	totalMinor := total.Decimal().Div(unit).Round(0)
	nDec := decimal.NewFromInt(int64(n))

	base := totalMinor.Div(nDec).Truncate(0)
	remainder := totalMinor.Sub(base.Mul(nDec))
	// Truncate rounds toward zero, so for a negative total the remainder is
	// negative too and the extra minor units are handed out in that direction.
	step := decimal.NewFromInt(1)
	if remainder.IsNegative() {
		step = decimal.NewFromInt(-1)
		remainder = remainder.Neg()
	}
	extra := remainder.IntPart()

	parts := make([]Money, 0, n)
	for i := 0; i < n; i++ {
		amountMinor := base
		if int64(i) < extra {
			amountMinor = amountMinor.Add(step)
		}
		parts = append(parts, NewMoney(amountMinor.Mul(unit), ccy))
	}

	// Conservation is an invariant, not a hope: assert it rather than trust the
	// arithmetic above.
	sum := Zero(ccy)
	for _, p := range parts {
		sum = sum.Add(p)
	}
	if !sum.Equal(total) {
		return nil, apperror.New(apperror.ErrInternalValidation,
			"instalments sum to "+sum.String()+", expected "+total.String())
	}
	return parts, nil
}
