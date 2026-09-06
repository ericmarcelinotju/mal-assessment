package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
)

func TestSplitInstalments_ConservesValue(t *testing.T) {
	t.Run("when BHD 10.000 is split three ways then the parts sum to 10.000", func(t *testing.T) {
		parts, err := ledger.SplitInstalments(bhd("10.000"), 3)
		assert.NoError(t, err)
		assert.Len(t, parts, 3)

		assert.Equal(t, "3.334", parts[0].String())
		assert.Equal(t, "3.333", parts[1].String())
		assert.Equal(t, "3.333", parts[2].String())

		sum := entity.Zero(entity.BHD)
		for _, p := range parts {
			sum = sum.Add(p)
		}
		assertMoney(t, bhd("10.000"), sum, "")
	})
}

// TestSplitInstalments_RefutesCriterion7 is the arithmetic behind refusing
// acceptance criterion 7, which asserts the three instalments must each be
// BHD 3.334.
func TestSplitInstalments_RefutesCriterion7(t *testing.T) {
	t.Run("when each instalment is 3.334 then the credit becomes 10.002", func(t *testing.T) {
		three := bhd("3.334").Add(bhd("3.334")).Add(bhd("3.334"))
		assert.Equal(t, "10.002", three.String(),
			"criterion 7 credits 0.002 that nobody instructed")
		assert.False(t, three.Equal(bhd("10.000")))
	})

	t.Run("when each instalment is 3.333 then the credit becomes 9.999", func(t *testing.T) {
		three := bhd("3.333").Add(bhd("3.333")).Add(bhd("3.333"))
		assert.Equal(t, "9.999", three.String(),
			"the other equal split loses 0.001 that was instructed")
	})

	// Three genuinely equal 3dp instalments summing to 10.000 do not exist:
	// 10.000 is not divisible by 3 in thousandths. The brief asks for equality
	// and for a total of 10.000, and only one of the two can hold. Conservation
	// wins, because 10.000 is the credit while equality only describes how it
	// is delivered.
	t.Run("when equality and conservation conflict then conservation wins", func(t *testing.T) {
		parts, err := ledger.SplitInstalments(bhd("10.000"), 3)
		assert.NoError(t, err)

		spread := parts[0].Sub(parts[2])
		assertMoney(t, bhd("0.001"), spread,
			"parts differ by exactly one minor unit -- the minimum possible")
	})
}

func TestSplitInstalments_EdgeCases(t *testing.T) {
	t.Run("when the amount divides evenly then every part is identical", func(t *testing.T) {
		parts, err := ledger.SplitInstalments(bhd("9.000"), 3)
		assert.NoError(t, err)
		for _, p := range parts {
			assert.Equal(t, "3.000", p.String())
		}
	})

	t.Run("when the amount is negative then the residual goes the same way", func(t *testing.T) {
		parts, err := ledger.SplitInstalments(bhd("-10.000"), 3)
		assert.NoError(t, err)

		sum := entity.Zero(entity.BHD)
		for _, p := range parts {
			sum = sum.Add(p)
		}
		assertMoney(t, bhd("-10.000"), sum, "a negative split must conserve too")
		assert.Equal(t, "-3.334", parts[0].String())
	})

	t.Run("when the count is one then the amount passes through", func(t *testing.T) {
		parts, err := ledger.SplitInstalments(aed("100.00"), 1)
		assert.NoError(t, err)
		assert.Len(t, parts, 1)
		assertMoney(t, aed("100.00"), parts[0], "")
	})

	t.Run("when the count is not positive then it is refused", func(t *testing.T) {
		_, err := ledger.SplitInstalments(aed("100.00"), 0)
		assert.Error(t, err)
	})

	// A split that cannot conserve value is a bug in the allocator, so the
	// allocator asserts conservation rather than trusting its own arithmetic.
	// Exhaustive over awkward divisors at both scales.
	t.Run("when splitting many amounts many ways then every split conserves", func(t *testing.T) {
		for _, amount := range []entity.Money{
			bhd("10.000"), bhd("0.001"), bhd("100.007"), aed("0.01"), aed("999.99"),
		} {
			for n := 1; n <= 13; n++ {
				parts, err := ledger.SplitInstalments(amount, n)
				assert.NoError(t, err)

				sum := entity.Zero(amount.Currency())
				for _, p := range parts {
					sum = sum.Add(p)
				}
				assertMoney(t, amount, sum, "splitting %s into %d parts", amount.Display(), n)
			}
		}
	})
}
