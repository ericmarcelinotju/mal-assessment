package replay_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

func TestInterest_DailyAccruals(t *testing.T) {
	t.Run("when the window closes then each day accrues on its final balance", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		want := map[entity.Day]string{
			1: "0.10", // 250.00
			2: "0.09", // 225.00
			3: "0.25", // 625.00
			4: "0.17", // 415.00
			5: "0.16", // 390.00
			6: "0.16", // 390.00
		}
		for day, amount := range want {
			assertMoney(t, aed(amount), row(t, s, replay.ACC001, day).InterestAccrued,
				"day %d", day)
		}
	})

	t.Run("when the balance is negative then nothing accrues", func(t *testing.T) {
		// A negative balance is priced by the overdraft fee. Charging interest
		// on it as well would be charging twice for one condition, and the brief
		// says positive balances only.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		s := newStack(t, config.FeeReversalNone, accounts...)
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
		}))

		for day := entity.Day(1); day <= 6; day++ {
			assertMoney(t, aed("0.00"), row(t, s, "A", day).InterestAccrued, "day %d", day)
		}
	})
}

// TestInterest_RestatesBackValuedDays is the append-only correction mechanism.
func TestInterest_RestatesBackValuedDays(t *testing.T) {
	t.Run("when a back-valued entry lands then the day 2 trail records every belief", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)
		trail := row(t, s, replay.ACC001, 2).AccrualTrail

		assert.Len(t, trail, 3, "one original and two corrections")

		// Booked on day 2, when the balance was still 250.00.
		assertMoney(t, aed("0.10"), trail[0].Amount, "")
		assert.Equal(t, entity.Day(2), trail[0].BookedOnDay)
		assertMoney(t, aed("250.00"), trail[0].Basis, "")

		// Corrected on day 5, when E7 arrived back-valued and day 2 went to
		// -395.00 -- a negative balance accrues nothing, so the whole 0.10 is
		// taken back.
		assertMoney(t, aed("-0.10"), trail[1].Amount, "")
		assert.Equal(t, entity.Day(5), trail[1].BookedOnDay)
		assertMoney(t, aed("-395.00"), trail[1].Basis, "")

		// Corrected again on day 6, when E9 reversed E7 and day 2 settled at
		// 225.00. Not back to 0.10: the overdraft fee is still there.
		assertMoney(t, aed("0.09"), trail[2].Amount, "")
		assert.Equal(t, entity.Day(6), trail[2].BookedOnDay)
		assertMoney(t, aed("225.00"), trail[2].Basis, "")
	})

	t.Run("when accruals are corrected then no record is rewritten", func(t *testing.T) {
		// The correction is appended. The original +0.10 still reads exactly as
		// it was written on day 2, which is what lets an auditor reconstruct
		// what the ledger believed and when.
		s := run(t, config.FeeReversalNone)

		for _, a := range accruals(t, s) {
			assert.NotZero(t, a.Seq, "every accrual record carries its append position")
		}

		trail := row(t, s, replay.ACC001, 2).AccrualTrail
		assert.Less(t, trail[0].Seq, trail[1].Seq, "corrections come after what they correct")
		assert.Less(t, trail[1].Seq, trail[2].Seq)
	})

	t.Run("when the trail is summed then it equals the day's net accrual", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		for day := entity.Day(1); day <= 6; day++ {
			r := row(t, s, replay.ACC001, day)
			sum := entity.Zero(entity.AED)
			for _, a := range r.AccrualTrail {
				sum = mustAdd(t, sum, a.Amount)
			}
			assertMoney(t, r.InterestAccrued, sum, "day %d trail must sum to its net", day)
		}
	})
}

// TestInterest_CapitalisationSumsExactly is the non-negotiable rule that the
// rounded daily accruals must sum exactly to the capitalised total, and the
// refutation of acceptance criterion 8.
func TestInterest_CapitalisationSumsExactly(t *testing.T) {
	t.Run("when interest is capitalised then it equals the sum of the dailies", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		daily := entity.Zero(entity.AED)
		for day := entity.Day(1); day <= 6; day++ {
			daily = mustAdd(t, daily, row(t, s, replay.ACC001, day).InterestAccrued)
		}
		assertMoney(t, aed("0.93"), daily, "")

		capitalisation := row(t, s, replay.ACC001, 6).Capitalisation
		assert.NotNil(t, capitalisation, "a single credit, on the capitalisation day")
		assertMoney(t, daily, capitalisation.Amount,
			"the rule is exact equality, not approximate agreement")
	})

	t.Run("when it is booked then it is one credit, not six", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		var count int
		for _, e := range entries(t, s) {
			if e.AccountID == replay.ACC001 && e.Origin == entity.OriginCapitalisation {
				count++
				assert.Equal(t, entity.Day(6), e.ValueDate)
			}
		}
		assert.Equal(t, 1, count)
	})

	// This is where criterion 8 bites. ACC-001's exact unrounded interest is
	// 0.9180, which rounds to 0.92, while the rounded dailies sum to 0.93. The
	// two routes disagree by a cent on this very stream, so "discard the
	// remainder" is not a harmless clause -- it opens a permanent one-cent break
	// between the accrual subledger and the capitalisation entry.
	t.Run("when the total is computed independently then it differs by 0.01", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		// Sum the daily interest WITHOUT rounding each term, then round once at
		// the end. This is the "compute the total independently" route that
		// criterion 8 presupposes.
		rate := decimal.New(4, -4) // exactly 0.0004
		exact := decimal.Zero
		for _, balance := range []string{"250.00", "225.00", "625.00", "415.00", "390.00", "390.00"} {
			exact = exact.Add(aed(balance).Decimal().Mul(rate))
		}
		assert.Equal(t, "0.918", exact.String(), "the exact unrounded interest")
		assert.Equal(t, "0.92", exact.Round(2).String(), "which rounds to 0.92")

		// The engine capitalises the sum of the ROUNDED dailies, which is 0.93.
		// The two routes disagree by a cent on this very stream, so "discard the
		// remainder" is not a harmless clause: it would open a permanent
		// one-cent break between the accrual subledger and the capitalisation
		// entry.
		capitalisation := row(t, s, replay.ACC001, 6).Capitalisation
		assertMoney(t, aed("0.93"), capitalisation.Amount, "")
		assert.NotEqual(t, exact.Round(2).String(), capitalisation.Amount.String(),
			"the routes genuinely disagree; the rule requires the sum of the dailies")
	})

	t.Run("when the account is BHD then capitalisation is exact at three places", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		daily := entity.Zero(entity.BHD)
		for day := entity.Day(1); day <= 6; day++ {
			daily = mustAdd(t, daily, row(t, s, replay.ACC002, day).InterestAccrued)
		}
		assertMoney(t, bhd("0.008"), daily, "0.004 on day 5 and 0.004 on day 6")

		capitalisation := row(t, s, replay.ACC002, 6).Capitalisation
		assert.NotNil(t, capitalisation)
		assertMoney(t, daily, capitalisation.Amount, "")
	})
}

func TestInterest_CapitalisationDoesNotFeedItself(t *testing.T) {
	t.Run("when interest is capitalised then day 6 accrues on the pre-credit balance", func(t *testing.T) {
		// The day 6 credit is interest ON the window, not part of it. Letting it
		// into its own accrual, or into the day 6 overdraft test, would be
		// circular. Day 6 closes at 390.93 but accrues on 390.00.
		s := run(t, config.FeeReversalNone)
		day6 := row(t, s, replay.ACC001, 6)

		assertMoney(t, aed("390.93"), day6.ClosingBalance, "")
		assertMoney(t, aed("0.16"), day6.InterestAccrued,
			"0.16 is 390.00 x 0.0004; 390.93 would have given 0.16 too, but the "+
				"basis must be the pre-credit balance regardless")
	})
}

func TestInterest_UnderFeeReversalPolicy(t *testing.T) {
	t.Run("when fees are reversed then interest restates upward with them", func(t *testing.T) {
		// The reversal sweep runs before accrual in the day close, so interest
		// is computed on the post-sweep balances. Every day lands on the pre-E7
		// counterfactual.
		s := run(t, config.FeeReversalOnCauseReversal)

		want := map[entity.Day]string{
			1: "0.10", 2: "0.10", 3: "0.26", 4: "0.19", 5: "0.19", 6: "0.19",
		}
		total := entity.Zero(entity.AED)
		for day, amount := range want {
			assertMoney(t, aed(amount), row(t, s, replay.ACC001, day).InterestAccrued, "day %d", day)
		}
		for day := entity.Day(1); day <= 6; day++ {
			total = mustAdd(t, total, row(t, s, replay.ACC001, day).InterestAccrued)
		}
		assertMoney(t, aed("1.03"), total, "")
		assertMoney(t, aed("1.03"), row(t, s, replay.ACC001, 6).Capitalisation.Amount,
			"sum-exactness holds under either policy")
	})
}
