package replay_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// TestScenario_DefaultPolicy pins every figure the six-day replay produces
// under the literal reading of the brief. If any rule changes, this fails with
// the exact day and account that moved.
func TestScenario_DefaultPolicy(t *testing.T) {
	s := run(t, config.FeeReversalNone)

	t.Run("ACC-001 closing balances", func(t *testing.T) {
		want := map[entity.Day]string{
			1: "250.00",
			2: "225.00", // E7 and E9 cancel; the day 2 fee remains
			3: "625.00",
			4: "415.00", // E6 rejected, so no 180.00 debit
			5: "390.00",
			6: "390.93", // includes the 0.93 capitalisation
		}
		for day, amount := range want {
			assertMoney(t, aed(amount), row(t, s, replay.ACC001, day).ClosingBalance, "day %d", day)
		}
	})

	t.Run("ACC-001 fees", func(t *testing.T) {
		assessed, reversed := countFees(t, s, replay.ACC001)
		assert.Equal(t, 3, assessed)
		assert.Equal(t, 0, reversed)

		total := entity.Zero(entity.AED)
		for _, e := range entries(t, s) {
			if e.AccountID == replay.ACC001 && e.IsFee() {
				total = mustAdd(t, total, e.Amount)
			}
		}
		assertMoney(t, aed("-75.00"), total, "")
	})

	t.Run("ACC-001 interest", func(t *testing.T) {
		assertMoney(t, aed("0.93"), row(t, s, replay.ACC001, 6).Capitalisation.Amount, "")
	})

	t.Run("ACC-001 holds and availability", func(t *testing.T) {
		want := map[entity.Day]struct{ holds, available string }{
			1: {"0.00", "250.00"},
			2: {"200.00", "25.00"},
			3: {"200.00", "425.00"},
			4: {"0.00", "415.00"},
			5: {"0.00", "390.00"},
			6: {"0.00", "390.93"},
		}
		for day, w := range want {
			r := row(t, s, replay.ACC001, day)
			assertMoney(t, aed(w.holds), r.ActiveHolds, "day %d holds", day)
			assertMoney(t, aed(w.available), r.AvailableBalance, "day %d available", day)
		}
	})

	t.Run("ACC-002 closing balances", func(t *testing.T) {
		want := map[entity.Day]string{
			1: "0.000", 2: "0.000", 3: "0.000", 4: "0.000",
			5: "10.000",
			6: "10.008", // includes the 0.008 capitalisation
		}
		for day, amount := range want {
			assertMoney(t, bhd(amount), row(t, s, replay.ACC002, day).ClosingBalance, "day %d", day)
		}

		assessed, _ := countFees(t, s, replay.ACC002)
		assert.Equal(t, 0, assessed, "ACC-002 is never negative")
	})

	t.Run("errors", func(t *testing.T) {
		// Exactly two instructions are refused, and both are refusals the brief
		// engineered: an orphan settlement and an unfundable authorization.
		assert.Len(t, journalErrors(t, s), 2)

		codes := map[string]bool{}
		for _, e := range journalErrors(t, s) {
			codes[e.Code] = true
		}
		assert.True(t, codes["2002"], "E6: orphan settlement")
		assert.True(t, codes["2005"], "E8: insufficient funds")
	})
}

// TestScenario_FeeReversalPolicy pins the alternative reading, in which fees
// are de-assessed once their cause is reversed. Every figure lands on the
// pre-E7 counterfactual: this is what acceptance criterion 6 asserts, and it is
// true here and only here.
func TestScenario_FeeReversalPolicy(t *testing.T) {
	s := run(t, config.FeeReversalOnCauseReversal)

	t.Run("ACC-001 closing balances return to the pre-E7 path", func(t *testing.T) {
		want := map[entity.Day]string{
			1: "250.00",
			2: "250.00",
			3: "650.00",
			4: "465.00",
			5: "465.00",
			6: "466.03", // includes the 1.03 capitalisation
		}
		for day, amount := range want {
			assertMoney(t, aed(amount), row(t, s, replay.ACC001, day).ClosingBalance, "day %d", day)
		}
	})

	t.Run("fees net to zero", func(t *testing.T) {
		total := entity.Zero(entity.AED)
		for _, e := range entries(t, s) {
			if e.AccountID == replay.ACC001 && (e.IsFee() || e.IsFeeReversal()) {
				total = mustAdd(t, total, e.Amount)
			}
		}
		assertMoney(t, aed("0.00"), total, "three fees assessed, three reversed")
	})

	t.Run("interest is 1.03", func(t *testing.T) {
		assertMoney(t, aed("1.03"), row(t, s, replay.ACC001, 6).Capitalisation.Amount, "")
	})

	t.Run("ACC-002 is unaffected by the policy", func(t *testing.T) {
		assertMoney(t, bhd("10.008"), row(t, s, replay.ACC002, 6).ClosingBalance,
			"ACC-002 never went negative, so no fee policy can touch it")
	})
}

// TestScenario_PolicyDelta states the cost of the choice in one place.
func TestScenario_PolicyDelta(t *testing.T) {
	t.Run("when the policies are compared then they differ by 75.10", func(t *testing.T) {
		none := row(t, run(t, config.FeeReversalNone), replay.ACC001, 6).ClosingBalance
		reversal := row(t, run(t, config.FeeReversalOnCauseReversal), replay.ACC001, 6).ClosingBalance

		assertMoney(t, aed("75.10"), mustSub(t, reversal, none),
			"75.00 of fees plus 0.10 of interest those fees cost the account")
	})
}
