package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// TestBalance_IsBitemporal is the heart of the exercise.
//
// E7 posts on day 5 carrying value date day 2. "The day 2 closing balance" is
// therefore not a number but a function of two coordinates: the day being
// measured, and the day it is measured from. Both readings below are correct,
// and any implementation that can only produce one of them has lost
// information the ledger genuinely holds.
func TestBalance_IsBitemporal(t *testing.T) {
	t.Run("when day 2 is observed from day 2 then it closes at 250.00", func(t *testing.T) {
		// Only E1 and E2 have been posted. E7 does not exist yet, so nothing
		// about the account is negative and no fee is due.
		cfg := config.Default()
		svc := ledger.New(cfg, ledger.CanonicalAccounts()...)

		var upToDay2 []entity.Event
		for _, ev := range ledger.CanonicalStream() {
			if ev.PostingDay <= 2 {
				upToDay2 = append(upToDay2, ev)
			}
		}
		assert.NoError(t, svc.Replay(upToDay2))

		assertMoney(t, aed("250.00"), row(t, svc, ledger.ACC001, 2).ClosingBalance, "")
	})

	t.Run("when day 2 is observed from day 6 then it closes at 225.00", func(t *testing.T) {
		// E7 (-620.00) and E9 (+620.00) both value-date to day 2 and cancel, but
		// the overdraft fee E7 triggered remains, so day 2 ends 25.00 short of
		// where it began.
		svc := replay(t, config.FeeReversalNone)
		assertMoney(t, aed("225.00"), row(t, svc, ledger.ACC001, 2).ClosingBalance, "")
	})
}

// TestBalance_BackValuedEntryDepressesEveryLaterDay is the mechanism that makes
// criterion 2 unsatisfiable, isolated from the fee logic.
func TestBalance_BackValuedEntryDepressesEveryLaterDay(t *testing.T) {
	t.Run("when a day-2 debit is posted then days 2 through 5 all fall by it", func(t *testing.T) {
		// The rule defines a day's closing balance as every entry with
		// value_date <= that day. A day-2 entry is a member of that set for
		// day 2 and for every later day, so it cannot depress one day alone.
		cfg := config.Default()
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}

		before := ledger.New(cfg, accounts...)
		assert.NoError(t, before.Replay([]entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("1000.00")},
		}))

		after := ledger.New(cfg, accounts...)
		assert.NoError(t, after.Replay([]entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("1000.00")},
			{ID: "D", Type: entity.EventDebit, PostingDay: 5, ValueDate: 2, AccountID: "A", Amount: aed("100.00")},
		}))

		// Days 2 to 5 only. Day 6 carries the interest capitalisation, and the
		// two runs accrue different interest precisely because their balances
		// differ -- so day 6 shows a delta of 100.20, not 100.00. That is the
		// design working, not a discrepancy: a back-valued debit costs the
		// account the debit plus the interest it would have earned.
		for day := entity.Day(2); day <= 5; day++ {
			delta := row(t, before, "A", day).ClosingBalance.Sub(row(t, after, "A", day).ClosingBalance)
			assertMoney(t, aed("100.00"), delta, "day %d must fall by the back-valued debit", day)
		}

		day6 := row(t, before, "A", 6).ClosingBalance.Sub(row(t, after, "A", 6).ClosingBalance)
		assertMoney(t, aed("100.20"), day6,
			"day 6 also carries the 0.20 of interest the debit cost the account")

		// Day 1 precedes the value date, so it is untouched.
		assertMoney(t,
			row(t, before, "A", 1).ClosingBalance,
			row(t, after, "A", 1).ClosingBalance,
			"a day before the value date must not move")
	})
}

func TestBalance_HoldsDoNotMoveTheLedger(t *testing.T) {
	// The substance of criterion 5, tested on Auth-A, which is actually
	// approved. Auth-B, the authorization criterion 5 names, is declined -- see
	// TestAuthorization_AuthBIsDeclined.
	t.Run("when a hold is active then it cuts available but not ledger balance", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)
		day3 := row(t, svc, ledger.ACC001, 3)

		assertMoney(t, aed("625.00"), day3.ClosingBalance, "the hold must not touch the ledger")
		assertMoney(t, aed("200.00"), day3.ActiveHolds, "")
		assertMoney(t, aed("425.00"), day3.AvailableBalance, "")
		assertMoney(t, day3.ClosingBalance.Sub(day3.ActiveHolds), day3.AvailableBalance, "")
	})

	t.Run("when the hold settles then it stops reducing availability", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)
		day4 := row(t, svc, ledger.ACC001, 4)

		assertMoney(t, aed("0.00"), day4.ActiveHolds, "settlement releases the hold in full")
		assertMoney(t, day4.ClosingBalance, day4.AvailableBalance, "")
	})
}

func TestBalance_LogSumsToTheFinalPosition(t *testing.T) {
	// An independent check on the report: sum the raw log and compare. If the
	// report ever drifts from the entries that justify it, this fails.
	t.Run("when every entry is summed then it equals the day 6 closing balance", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		total := entity.Zero(entity.AED)
		for _, e := range svc.Entries() {
			if e.AccountID == ledger.ACC001 {
				total = total.Add(e.Amount)
			}
		}
		assertMoney(t, aed("390.93"), total, "")
		assertMoney(t, total, row(t, svc, ledger.ACC001, 6).ClosingBalance, "")
	})
}
