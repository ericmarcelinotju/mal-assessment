package replay_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// ============================================================================
// THE FAILING TEST
//
// This test FAILS. That is deliberate, it is the only failing test in the
// suite, and `make test` is expected to report exactly one failure. `make
// verify` skips it (-skip 'TestKnownGap_') for a green run.
//
// It is not a bug left unfixed. It is an argument against the specification,
// written in the only language a specification cannot talk its way out of.
// ============================================================================

// TestKnownGap_OverdraftFeesSurviveReversalOfTheirCause asserts that ACC-001
// closes day 6 at AED 466.03 -- the balance it would have had if E7 had never
// been posted. It fails, reporting 390.93.
//
// # WHAT IT REVEALS
//
// 1. The rule set prices an outcome without asking who caused it.
//
//	E7 is a debit of 620.00, back-valued to day 2, which E9 reverses on day 6.
//	A reversal is what a ledger does when an entry should not have been made:
//	a duplicate, a mis-keyed amount, a posting to the wrong account. The brief
//	never says whose mistake E7 was, and the engine never asks -- but the
//	answer decides whether the 75.00 is a fee or a loss.
//
//	If the customer caused it, three overdraft fees are exactly right.
//	If the bank caused it, the customer has been charged 75.00 plus 0.10 of
//	foregone interest for an overdraft that, on the corrected record, never
//	happened. And the fees are permanent: the brief grants an assessment
//	primitive and no de-assessment primitive, so nothing in the specified rule
//	set can ever give the money back.
//
// 2. A fee has no causal link to what triggered it.
//
//	The engine records that day 2 was negative when it was assessed. It does
//	not record WHY, so when E9 arrives there is no way to ask "is the entry
//	that caused this fee still standing?" A LedgerEntry would need to carry the
//	sequence numbers that pushed its day negative for that question to be
//	answerable at all. Until then, "reverse the fees caused by E7" is not a
//	query this data model can express.
//
// 3. The gap is fixable, and the fix is in this repository.
//
//	config.FeeReversalOnCauseReversal implements de-assessment, and under it
//	this exact assertion passes -- see TestCriterion6_RefusedUnderDefaultPolicy,
//	which proves the policy reproduces the pre-E7 counterfactual day for day.
//	So this is not a hypothetical objection: the repair is written, tested, and
//	one environment variable away.
//
//	It is not the default, and this test stays red, because the brief's rules
//	are non-negotiable and they do not contain de-assessment. Making my own
//	extension the default would quietly overwrite the specification and hide
//	precisely the disagreement worth having. A red test states the objection
//	without pretending the objection is settled.
//
// 4. What the fix still would not settle.
//
//	on_cause_reversal reverses a fee whenever its day recovers, for ANY reason
//	-- including a genuine customer deposit days later. That is too generous:
//	an overdraft that really happened should still be charged. Getting this
//	right needs the causal link from point 2, so that a fee is reversed only
//	when the specific entry that caused it is reversed. The policy in this repo
//	lands on the right answer for this stream through a coarser rule, and a
//	reviewer should know the difference.
//
// WHAT WOULD MAKE IT PASS, PROPERLY
//
//	a. LedgerEntry gains CausedBySeq []int, populated at assessment with the
//	   entries that put the day under.
//	b. A FEE_REVERSAL is booked when every entry in that set has been reversed
//	   -- not merely when the balance happens to have recovered.
//	c. The brief gains a rule saying so, because (a) and (b) are policy, and
//	   policy is not mine to invent silently.
func TestKnownGap_OverdraftFeesSurviveReversalOfTheirCause(t *testing.T) {
	s := run(t, config.FeeReversalNone)

	// The pre-E7 path, computed independently rather than hardcoded: replay the
	// same stream with E7 and its reversal removed entirely.
	var withoutE7 []entity.Event
	for _, ev := range replay.CanonicalStream() {
		if ev.ID != "E7" && ev.ID != "E9" {
			withoutE7 = append(withoutE7, ev)
		}
	}
	counterfactual := newStack(t, config.FeeReversalNone, replay.CanonicalAccounts()...)
	assert.NoError(t, counterfactual.replay.Run(context.Background(), withoutE7))

	want := row(t, counterfactual, replay.ACC001, 6).ClosingBalance // 466.03
	got := row(t, s, replay.ACC001, 6).ClosingBalance               // 390.93

	// FAILS: 390.93, short by 75.10.
	//
	// A debit that was posted and then withdrawn has permanently cost the
	// account 75.00 in fees and 0.10 in interest it would otherwise have
	// earned. The ledger is internally consistent and every entry is
	// justifiable; it is the rule set that has no way to say sorry.
	assertMoney(t, want, got,
		"an entry that was reversed should leave no permanent charge behind")

	// The same failure stated as the fee count, so the diff is legible without
	// working backwards from a balance.
	assessed, reversed := countFees(t, s, replay.ACC001)
	assert.Equal(t, assessed, reversed,
		"every fee whose cause was reversed should itself have been reversed; "+
			"the default rule set has no primitive for that")
}
