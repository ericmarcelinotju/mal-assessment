package replay_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// TestFee_E7CausesThreeFees is the refutation of acceptance criterion 2, which
// claims E7 causes exactly one fee, on day 2.
func TestFee_E7CausesThreeFees(t *testing.T) {
	t.Run("when E7 is posted then days 2, 4 and 5 each take a fee", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		assessed, reversed := countFees(t, s, replay.ACC001)
		assert.Equal(t, 3, assessed, "criterion 2 claims one fee; there are three")
		assert.Equal(t, 0, reversed, "the default policy has no de-assessment primitive")

		for _, day := range []entity.Day{2, 4, 5} {
			a, _ := feesOn(t, s, replay.ACC001, day)
			assert.Equal(t, 1, a, "day %d must carry exactly one fee", day)
		}
		for _, day := range []entity.Day{1, 3, 6} {
			a, _ := feesOn(t, s, replay.ACC001, day)
			assert.Equal(t, 0, a, "day %d must carry no fee", day)
		}
	})

	t.Run("when all three fees are assessed then it happens on processing day 5", func(t *testing.T) {
		// The fees are value-dated 2, 4 and 5 but all become knowable at the
		// same moment: when E7 arrives on day 5. Value date and posting day are
		// different questions, and the log records both.
		s := run(t, config.FeeReversalNone)

		for _, e := range entries(t, s) {
			if e.IsFee() {
				assert.Equal(t, entity.Day(5), e.PostingDay,
					"fee for day %d was learned of on day 5", e.ValueDate)
			}
		}
	})

	t.Run("when the counterfactual has no E7 then there are no fees at all", func(t *testing.T) {
		// But-for causation: without E7 the account is never negative, so all
		// three fees are caused by E7. There is no reading on which it causes one.
		var withoutE7 []entity.Event
		for _, ev := range replay.CanonicalStream() {
			if ev.ID != "E7" && ev.ID != "E9" {
				withoutE7 = append(withoutE7, ev)
			}
		}
		s := newStack(t, config.FeeReversalNone, replay.CanonicalAccounts()...)
		assert.NoError(t, s.replay.Run(context.Background(), withoutE7))

		assessed, _ := countFees(t, s, replay.ACC001)
		assert.Equal(t, 0, assessed)
		assertMoney(t, aed("465.00"), row(t, s, replay.ACC001, 5).ClosingBalance, "")
	})
}

// TestFee_Criterion2BoundaryCase shows how narrowly criterion 2 fails.
//
// After E7, day 3's credit does restore the account to +5.00. What kills the
// criterion is E5: settling 185.00 on day 4 pushes it back under. Had Auth-A
// settled for 5.00 or less, day 4 would have stayed positive, day 5 with it,
// and E7 really would have caused exactly one fee dated day 2.
//
// The criterion is not absurd -- it is wrong about this stream by the size of
// one settlement. Demonstrating that is more honest than asserting it.
func TestFee_Criterion2BoundaryCase(t *testing.T) {
	build := func(settlement string) []entity.Event {
		var out []entity.Event
		for _, ev := range replay.CanonicalStream() {
			if ev.ID == "E5" {
				ev.Amount = aed(settlement)
			}
			out = append(out, ev)
		}
		return out
	}

	t.Run("when Auth-A settles 185.00 then three days go negative", func(t *testing.T) {
		s := newStack(t, config.FeeReversalNone, replay.CanonicalAccounts()...)
		assert.NoError(t, s.replay.Run(context.Background(), build("185.00")))

		assessed, _ := countFees(t, s, replay.ACC001)
		assert.Equal(t, 3, assessed)
	})

	t.Run("when Auth-A settles 5.00 then criterion 2 would have been right", func(t *testing.T) {
		// Day 3 closes at +5.00 after the day-2 fee; a 5.00 settlement leaves
		// day 4 at exactly 0.00, which is not negative, so no further fee falls.
		// One fee, dated day 2 -- exactly what criterion 2 asserts. The
		// criterion is not absurd; it is wrong about this stream by the size of
		// one settlement.
		s := newStack(t, config.FeeReversalNone, replay.CanonicalAccounts()...)
		assert.NoError(t, s.replay.Run(context.Background(), build("5.00")))

		assessed, _ := countFees(t, s, replay.ACC001)
		assert.Equal(t, 1, assessed, "exactly one fee, and it is dated day 2")

		a, _ := feesOn(t, s, replay.ACC001, 2)
		assert.Equal(t, 1, a)
	})
}

// TestFee_CascadeIsCoveredSynthetically fills a real gap.
//
// The engine sweeps days ascending so a fee booked on an earlier day is already
// in the balance when a later day is tested -- fees are ledger entries and
// compound like any other. On the canonical stream that choice changes nothing:
// the day-2 fee narrows day 3 from +30.00 to +5.00, which is still positive. So
// the brief's own data cannot tell ascending-with-cascade apart from evaluating
// every day against pre-fee balances. This case can.
func TestFee_CascadeIsCoveredSynthetically(t *testing.T) {
	t.Run("when a fee pushes the next day negative then that day is charged too", func(t *testing.T) {
		// Day 1 closes at -1.00 and takes a 25.00 fee, leaving -26.00. Day 2's
		// credit of 10.00 would have lifted a -1.00 balance to +9.00, but
		// against -26.00 it only reaches -16.00, so day 2 is charged as well.
		// Without the cascade, day 2 would close positive and escape.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		s := newStack(t, config.FeeReversalNone, accounts...)
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("1.00")},
			{ID: "C", Type: entity.EventCredit, PostingDay: 2, ValueDate: 2, AccountID: "A", Amount: aed("10.00")},
		}))

		d1, _ := feesOn(t, s, "A", 1)
		d2, _ := feesOn(t, s, "A", 2)
		assert.Equal(t, 1, d1, "day 1 closes at -1.00")
		assert.Equal(t, 1, d2, "day 2 closes at -16.00 only because of the day 1 fee")
		assertMoney(t, aed("-41.00"), row(t, s, "A", 2).ClosingBalance, "")
	})
}

func TestFee_AtMostOncePerAccountPerDay(t *testing.T) {
	t.Run("when a day stays negative across closes then it is charged once", func(t *testing.T) {
		// Day 2 is negative at the day-5 close and still negative at the day-6
		// close. Without the cap it would be charged twice.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		s := newStack(t, config.FeeReversalNone, accounts...)
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 2, ValueDate: 2, AccountID: "A", Amount: aed("100.00")},
		}))

		a, _ := feesOn(t, s, "A", 2)
		assert.Equal(t, 1, a, "one fee per account per day, however many closes run")
	})
}

func TestFee_IsDenominatedInTheAccountCurrency(t *testing.T) {
	// The brief states the fee as "AED 25.00" while ACC-002 is a BHD account.
	// An AED entry cannot be booked to a BHD account without a rate the brief
	// never supplies, so the fee is 25 units of the account's own currency. The
	// canonical stream never overdraws ACC-002, so this is the only place the
	// decision is exercised. See NUMBERS.md.
	t.Run("when a BHD account is overdrawn then the fee is BHD 25.000", func(t *testing.T) {
		accounts := []entity.Account{entity.NewAccount("B", entity.BHD, "0.000")}
		s := newStack(t, config.FeeReversalNone, accounts...)
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1, AccountID: "B", Amount: bhd("1.000")},
		}))

		var fee entity.LedgerEntry
		for _, e := range entries(t, s) {
			if e.IsFee() {
				fee = e
			}
		}
		assert.Equal(t, entity.BHD, fee.Amount.Currency(),
			"a fee must never be booked in a currency the account does not hold")
		assertMoney(t, bhd("-25.000"), fee.Amount, "")
	})
}

// TestFee_ReversalPolicy covers the criterion-6 branch.
func TestFee_ReversalPolicy(t *testing.T) {
	t.Run("when the policy is none then fees survive the reversal of their cause", func(t *testing.T) {
		s := run(t, config.FeeReversalNone)

		assessed, reversed := countFees(t, s, replay.ACC001)
		assert.Equal(t, 3, assessed)
		assert.Equal(t, 0, reversed)
		assertMoney(t, aed("390.93"), row(t, s, replay.ACC001, 6).ClosingBalance, "")
	})

	t.Run("when the policy is on_cause_reversal then all three fees reverse", func(t *testing.T) {
		s := run(t, config.FeeReversalOnCauseReversal)

		assessed, reversed := countFees(t, s, replay.ACC001)
		assert.Equal(t, 3, assessed, "the fees were correctly assessed when the days were negative")
		assert.Equal(t, 3, reversed, "and correctly undone once E9 removed the cause")

		// The reversals reach a fixed point on the pre-E7 counterfactual.
		assertMoney(t, aed("465.00"), row(t, s, replay.ACC001, 5).ClosingBalance, "")
		assertMoney(t, aed("466.03"), row(t, s, replay.ACC001, 6).ClosingBalance, "")
	})

	t.Run("when a fee is reversed then the original entry is untouched", func(t *testing.T) {
		// Append-only holds under this policy too: the reversal is a contra
		// entry, not an edit or a deletion.
		s := run(t, config.FeeReversalOnCauseReversal)

		var fees, reversals int
		for _, e := range entries(t, s) {
			switch {
			case e.IsFee():
				fees++
				assertMoney(t, aed("-25.00"), e.Amount, "the original fee still reads as it was written")
			case e.IsFeeReversal():
				reversals++
				assert.NotZero(t, e.ReversesSeq, "a reversal must name the entry it undoes")
			}
		}
		assert.Equal(t, 3, fees)
		assert.Equal(t, 3, reversals)
	})

	t.Run("when a day is still negative then its fee is not reversed", func(t *testing.T) {
		// Guards the exclusion rule: a fee is tested against its day's balance
		// WITHOUT that fee. A naive inclusive test would never reverse anything,
		// and an over-eager one would reverse fees on days that are genuinely
		// overdrawn. Here nothing is reversed, because nothing is put right.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		cfg := config.Default()
		cfg.FeeReversal = config.FeeReversalOnCauseReversal
		s := newStack(t, cfg.FeeReversal, accounts...)
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
		}))

		assessed, reversed := countFees(t, s, "A")
		assert.Equal(t, 6, assessed, "every day of the window closes negative")
		assert.Equal(t, 0, reversed, "none of them is put right, so none is reversed")
	})
}

// TestFee_SweepDoesNotOscillate pins the property that makes the fixed-point
// loop safe.
//
// Assessment tests the fee-inclusive balance and requires it negative; reversal
// tests the fee-exclusive balance and requires it non-negative. No single state
// satisfies both for the same day, so the loop cannot cycle. If a future rule
// change breaks that, the iteration cap turns an infinite loop into an error --
// and this test turns it into a failure first.
func TestFee_SweepDoesNotOscillate(t *testing.T) {
	t.Run("when a day is repeatedly closed then it settles on one outcome", func(t *testing.T) {
		cfg := config.Default()
		cfg.FeeReversal = config.FeeReversalOnCauseReversal
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		s := newStack(t, cfg.FeeReversal, accounts...)

		// A debit that overdraws, then a reversal that puts it right.
		assert.NoError(t, s.replay.Run(context.Background(), []entity.Event{
			{ID: "D", Type: entity.EventDebit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("50.00")},
			{ID: "C", Type: entity.EventCredit, PostingDay: 2, ValueDate: 1, AccountID: "A", Amount: aed("50.00")},
		}))

		assessed, reversed := countFees(t, s, "A")
		assert.Equal(t, assessed, reversed, "every fee assessed was also reversed exactly once")
		assertMoney(t, aed("0.00"), row(t, s, "A", 6).ClosingBalance,
			"the account returns to zero, with no residue from the fee cycle")

		// Closing the same day again must be idempotent.
		before := len(entries(t, s))
		assert.NoError(t, s.replay.CloseDay(context.Background(), 6))
		assert.Equal(t, before, len(entries(t, s)), "a repeated close must add nothing")
	})
}
