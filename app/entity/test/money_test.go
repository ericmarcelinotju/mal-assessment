package entity_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

func TestMoney_ScaleIsPerCurrency(t *testing.T) {
	t.Run("when the currency is BHD then amounts render at three places", func(t *testing.T) {
		assert.Equal(t, "10.000", bhd("10.000").String())
		assert.Equal(t, "10.000", bhd("10").String())
	})

	t.Run("when the currency is AED then amounts render at two places", func(t *testing.T) {
		assert.Equal(t, "250.00", aed("250").String())
		assert.Equal(t, "-370.00", aed("-370.00").String())
	})

	t.Run("when the scale is unknown then it is not defaulted", func(t *testing.T) {
		_, ok := entity.Currency("JPY").Scale()
		assert.False(t, ok, "an unknown currency must not silently take a scale")
	})
}

func TestMoney_RejectsOverPreciseInput(t *testing.T) {
	// Input is rejected rather than rounded. Rounding an instruction is how a
	// ledger loses money it was explicitly told about; rounding a computed
	// value (see MulRatioHalfUp) is a different matter and is allowed.
	t.Run("when input exceeds the currency scale then it is refused", func(t *testing.T) {
		_, err := entity.ParseMoney("3.3334", entity.BHD)
		assert.Error(t, err)

		_, err = entity.ParseMoney("1.005", entity.AED)
		assert.Error(t, err)
	})

	t.Run("when input is within the currency scale then it is accepted", func(t *testing.T) {
		m, err := entity.ParseMoney("3.334", entity.BHD)
		assert.NoError(t, err)
		assert.Equal(t, "3.334", m.String())
	})
}

func TestMoney_MulRatioHalfUp(t *testing.T) {
	// Every interest figure the six-day replay produces, checked against the
	// arithmetic done by hand. The rate is the exact rational 4/10000, so no
	// float64 is involved at any point.
	cases := []struct {
		balance  string
		currency entity.Currency
		want     string
		note     string
	}{
		{"250.00", entity.AED, "0.10", "0.1000 exact"},
		{"225.00", entity.AED, "0.09", "0.0900 exact"},
		{"625.00", entity.AED, "0.25", "0.2500 exact"},
		{"415.00", entity.AED, "0.17", "0.1660 rounds up"},
		{"390.00", entity.AED, "0.16", "0.1560 rounds up"},
		{"465.00", entity.AED, "0.19", "0.1860 rounds up"},
		{"650.00", entity.AED, "0.26", "0.2600 exact"},
		{"10.000", entity.BHD, "0.004", "0.0040 exact at three places"},
	}
	for _, c := range cases {
		t.Run(c.balance+" "+c.currency.String()+" -- "+c.note, func(t *testing.T) {
			got := entity.MustParseMoney(c.balance, c.currency).MulRatioHalfUp(4, 10000)
			assert.Equal(t, c.want, got.String())
		})
	}
}

func TestMoney_HalfUpRoundsAwayFromZero(t *testing.T) {
	// The rounding mode is documented in NUMBERS.md but is not load-bearing on
	// the canonical stream: no accrual there lands exactly on a half. This test
	// pins the mode anyway, so a future switch to banker's rounding is a
	// deliberate act with a failing test attached rather than a silent drift.
	t.Run("when the result is exactly a half then it rounds away from zero", func(t *testing.T) {
		// 0.125 at 2dp: half-up gives 0.13, banker's rounding would give 0.12.
		half := entity.NewMoney(decimal.RequireFromString("0.125"), entity.AED)
		assert.Equal(t, "0.13", half.String())

		negHalf := entity.NewMoney(decimal.RequireFromString("-0.125"), entity.AED)
		assert.Equal(t, "-0.13", negHalf.String())
	})
}

func TestMoney_CurrencyMismatchPanics(t *testing.T) {
	// Mixing currencies on one account is a programming error, not a runtime
	// condition. Returning an error would make every call site carry a check it
	// can only ignore; panicking makes the bug loud and immediate.
	t.Run("when adding across currencies then it panics", func(t *testing.T) {
		assert.Panics(t, func() { _ = aed("1.00").Add(bhd("1.000")) })
	})
}
