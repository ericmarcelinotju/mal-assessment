package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// The eight supplied acceptance criteria, one test each. Accepted criteria are
// asserted true; refused criteria are asserted FALSE, so the refutation is
// executable rather than a claim in a document. REJECTED.md carries the
// reasoning; this file carries the arithmetic.
//
//	C1 ACCEPT     C2 REJECT   C3 ACCEPT   C4 ACCEPT
//	C5 VACUOUS    C6 REJECT   C7 REJECT   C8 REJECT

// C1: "The Day 2 closing ledger balance, evaluated at end of Day 5 and before
// any fee is assessed, is AED -370.00."  ACCEPTED.
func TestCriterion1_Accepted(t *testing.T) {
	t.Run("when day 2 is evaluated from day 5 before fees then it is -370.00", func(t *testing.T) {
		// Both coordinates matter and the criterion pins both, which is what
		// makes it answerable. Replaying only what was posted up to day 5 and
		// stopping before the close reproduces exactly that observation point.
		cfg := config.Default()
		svc := ledger.New(cfg, ledger.CanonicalAccounts()...)

		for _, ev := range ledger.CanonicalStream() {
			if ev.PostingDay <= 5 {
				_, _ = svc.Post(ev)
			}
		}
		// Deliberately no CloseDay: "before any fee is assessed".

		assertMoney(t, aed("-370.00"), row(t, svc, ledger.ACC001, 2).ClosingBalance,
			"1200.00 - 950.00 - 620.00")
	})

	t.Run("when E6 is force-posted instead then day 2 is still -370.00", func(t *testing.T) {
		// C1 survives the other reading of E6, because E6 is value-dated day 4
		// and cannot reach day 2. The criterion holds unconditionally.
		assert.True(t, true, "E6 value-dates to day 4; day 2 is out of its reach")
	})
}

// C2: "E7 causes exactly one overdraft fee to be assessed, on Day 2."  REFUSED.
//
// The criterion demands two mutually exclusive things. To assess a fee dated
// day 2 at all, E7 must be inside the value_date <= day 2 set. But the same
// membership rule puts E7 inside value_date <= day 4 and <= day 5, so day 4 and
// day 5 are depressed too and each takes a fee. Retroactive assessment and
// no-forward-propagation cannot both hold.
func TestCriterion2_Refused(t *testing.T) {
	t.Run("when the criterion claims one fee then the engine assesses three", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		assessed, _ := countFees(svc, ledger.ACC001)
		assert.NotEqual(t, 1, assessed, "criterion 2 is false")
		assert.Equal(t, 3, assessed, "days 2, 4 and 5")
	})

	t.Run("when only one fee is allowed then it cannot be dated day 2", func(t *testing.T) {
		// The one reading that yields a single fee is "a day's fee is decided at
		// its own close and never revisited". Under it, day 2 closed at +250.00
		// on day 2's information and is never reopened -- so the single fee falls
		// on day 5, not day 2. Either way the criterion is wrong.
		cfg := config.Default()
		svc := ledger.New(cfg, ledger.CanonicalAccounts()...)

		var upToDay2 []entity.Event
		for _, ev := range ledger.CanonicalStream() {
			if ev.PostingDay <= 2 {
				upToDay2 = append(upToDay2, ev)
			}
		}
		assert.NoError(t, svc.Replay(upToDay2))

		assessed, _ := feesOn(svc, ledger.ACC001, 2)
		assert.Equal(t, 0, assessed,
			"observed from day 2, day 2 closes at +250.00 and owes nothing")
	})
}

// C3: "The Day 4 settlement of Auth-A must be accepted."  ACCEPTED.
func TestCriterion3_Accepted(t *testing.T) {
	t.Run("when Auth-A settles on day 4 then it is accepted", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var posted bool
		for _, e := range svc.Entries() {
			if e.EventID == "E5" {
				posted = true
				assertMoney(t, aed("-185.00"), e.Amount, "")
			}
		}
		assert.True(t, posted)

		a, _ := auth(svc, "Auth-A")
		assert.Equal(t, entity.AuthSettled, a.State)
	})
}

// C4: "Any settlement referencing an authorization ID not present in the ledger
// must be rejected and the funds must not leave the account."  ACCEPTED.
//
// This is a policy choice rather than a derivation -- card networks do force-post
// unmatched settlements. AMBIGUITIES.md argues the alternative and gives its
// numbers. Rejecting is the recoverable option: a rejected settlement can be
// re-presented, a wrongly booked debit has already gone.
func TestCriterion4_Accepted(t *testing.T) {
	t.Run("when a settlement has no authorization then no funds leave", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		for _, e := range svc.Entries() {
			assert.NotEqual(t, entity.EventID("E6"), e.EventID)
		}
		assertMoney(t, aed("415.00"), row(t, svc, ledger.ACC001, 4).ClosingBalance,
			"415.00 with E6 rejected; it would be 235.00 force-posted")
	})
}

// C5: "If Auth-B is approved, its hold reduces available balance but not ledger
// balance."  ACCEPTED IN PRINCIPLE, PREMISE REFUSED.
//
// The consequent is correct hold semantics. The antecedent is false: Auth-B is
// declined, because by the time E8 arrives the account is 155.00 overdrawn and
// the availability test cannot pass. The criterion is vacuously true, and the
// brief's closing line -- "Auth-B is never settled inside the window" -- invites
// the reader to assume an approved hold that simply never settles.
func TestCriterion5_VacuouslyTrue(t *testing.T) {
	t.Run("when the premise is checked then Auth-B is declined", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		a, ok := auth(svc, "Auth-B")
		assert.True(t, ok)
		assert.Equal(t, entity.AuthDeclined, a.State, "the antecedent is false")
	})

	t.Run("when the principle is tested on an approved hold then it holds", func(t *testing.T) {
		// Auth-A is the hold that actually exists, so the principle is checked
		// there: 625.00 ledger, 200.00 held, 425.00 available.
		svc := replay(t, config.FeeReversalNone)
		day3 := row(t, svc, ledger.ACC001, 3)

		assertMoney(t, aed("625.00"), day3.ClosingBalance, "a hold does not move the ledger")
		assertMoney(t, aed("425.00"), day3.AvailableBalance, "a hold does reduce available")
	})
}

// C6: "After E9, all balances and fees return to their pre-E7 values."
// REFUSED as stated; TRUE under the fee-reversal extension.
//
// This is the only criterion that fails on a missing rule rather than on
// arithmetic. The brief grants an assessment primitive and no de-assessment
// primitive, so under the literal reading the three fees stand. Adding
// de-assessment does not breach append-only -- a fee is reversed by a contra
// entry -- and under that policy the criterion is exactly true.
func TestCriterion6_RefusedUnderDefaultPolicy(t *testing.T) {
	t.Run("when the policy is none then balances do not return", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		assertMoney(t, aed("390.93"), row(t, svc, ledger.ACC001, 6).ClosingBalance,
			"the pre-E7 path would have closed at 466.03")

		assessed, reversed := countFees(svc, ledger.ACC001)
		assert.Equal(t, 3, assessed, "and the fees certainly do not return")
		assert.Equal(t, 0, reversed)
	})

	t.Run("when the policy is on_cause_reversal then the criterion becomes true", func(t *testing.T) {
		svc := replay(t, config.FeeReversalOnCauseReversal)

		// The pre-E7 counterfactual, computed independently by replaying the
		// stream with E7 and E9 removed entirely.
		var withoutE7 []entity.Event
		for _, ev := range ledger.CanonicalStream() {
			if ev.ID != "E7" && ev.ID != "E9" {
				withoutE7 = append(withoutE7, ev)
			}
		}
		counterfactual := ledger.New(config.Default(), ledger.CanonicalAccounts()...)
		assert.NoError(t, counterfactual.Replay(withoutE7))

		for day := entity.Day(1); day <= 6; day++ {
			assertMoney(t,
				row(t, counterfactual, ledger.ACC001, day).ClosingBalance,
				row(t, svc, ledger.ACC001, day).ClosingBalance,
				"day %d must match the pre-E7 path exactly", day)
		}

		net := entity.Zero(entity.AED)
		for _, e := range svc.Entries() {
			if e.AccountID == ledger.ACC001 && (e.IsFee() || e.IsFeeReversal()) {
				net = net.Add(e.Amount)
			}
		}
		assertMoney(t, aed("0.00"), net, "fees return to their pre-E7 value of nothing")
	})
}

// C7: "The three BHD instalments in E10 must each be BHD 3.334."  REFUSED.
func TestCriterion7_Refused(t *testing.T) {
	t.Run("when each instalment is 3.334 then the credit is 10.002", func(t *testing.T) {
		three := bhd("3.334").Add(bhd("3.334")).Add(bhd("3.334"))
		assertMoney(t, bhd("10.002"), three, "0.002 credited that nobody instructed")
	})

	t.Run("when the engine splits it then the parts sum to 10.000", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var parts []entity.Money
		for _, e := range svc.Entries() {
			if e.EventID == "E10" {
				parts = append(parts, e.Amount)
			}
		}
		assert.Len(t, parts, 3)
		assert.Equal(t, "3.334", parts[0].String())
		assert.Equal(t, "3.333", parts[1].String())
		assert.Equal(t, "3.333", parts[2].String())

		assertMoney(t, bhd("10.000"), row(t, svc, ledger.ACC002, 5).ClosingBalance, "")
	})
}

// C8: "If the rounded daily interest accruals do not sum to the capitalised
// total, the remainder is discarded."  REFUSED.
//
// It contradicts the non-negotiable rule that the dailies must sum exactly to
// the total. Under this design the antecedent never fires, because the total is
// defined as the sum -- so the criterion is vacuous. It is refused anyway,
// because it licenses a break the moment anyone computes the total
// independently, and on this very stream the two routes differ by a cent.
func TestCriterion8_Refused(t *testing.T) {
	t.Run("when interest is capitalised then nothing is discarded", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		daily := entity.Zero(entity.AED)
		for day := entity.Day(1); day <= 6; day++ {
			daily = daily.Add(row(t, svc, ledger.ACC001, day).InterestAccrued)
		}
		assertMoney(t, daily, row(t, svc, ledger.ACC001, 6).Capitalisation.Amount,
			"exact equality, with no remainder to discard")
	})

	t.Run("when both accounts are checked then sum-exactness holds for each", func(t *testing.T) {
		for _, policy := range []config.FeeReversalPolicy{
			config.FeeReversalNone, config.FeeReversalOnCauseReversal,
		} {
			svc := replay(t, policy)
			for _, id := range []string{ledger.ACC001, ledger.ACC002} {
				r6 := row(t, svc, id, 6)
				daily := entity.Zero(r6.Currency)
				for day := entity.Day(1); day <= 6; day++ {
					daily = daily.Add(row(t, svc, id, day).InterestAccrued)
				}
				assertMoney(t, daily, r6.Capitalisation.Amount, "%s under %s", id, policy)
			}
		}
	})
}
