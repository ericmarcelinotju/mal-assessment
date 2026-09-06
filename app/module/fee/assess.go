package fee

import (
	"context"
	"strconv"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// Assess sweeps every day of the window up to and including the processing day,
// charging the fee to any day whose closing ledger balance is negative and
// which has not been charged before.
//
// Two things make this a sweep rather than a single check on the current day.
//
// First, back-value postings. E7 arrives on Day 5 value-dated Day 2, so Day 2's
// closing balance becomes negative days after Day 2 ended. The fee rule is
// written in terms of "all entries with value_date <= that day", which is a
// statement about the value-dated balance, not about what was known at the
// time; the fee is therefore assessed when the ledger learns of the overdraft
// and booked with value_date equal to the day assessed.
//
// Second, and this is why criterion 2 is refutable: a back-value entry is a
// member of the value_date <= d set for its own day AND for every later day.
// Once E7 puts Day 2 at -370.00 it also puts Day 4 at -155.00 and Day 5 at
// -155.00. Accepting the Day 2 fee forces the Day 4 and Day 5 fees. There is no
// consistent reading in which E7 causes exactly one fee dated Day 2.
//
// The sweep runs ascending so that a fee booked on an earlier day is already in
// the balance when a later day is evaluated -- fees are ledger entries and
// compound like any other. On this stream the cascade changes no outcome (the
// Day 2 fee narrows Day 3 from +30.00 to +5.00, which is still positive), so
// the canonical events do not discriminate ascending-with-cascade from
// simultaneous evaluation. The synthetic cascade test covers that gap.
func (s *service) Assess(ctx context.Context, acc entity.Account, processingDay entity.Day) error {
	amount := s.amount(acc)

	for day := entity.Day(1); day <= processingDay; day++ {
		charged, err := s.repo.IsAssessed(ctx, acc.ID, day)
		if err != nil {
			return apperror.New(apperror.ErrUnexpected, "read fee assessment error", err)
		}
		if charged {
			continue
		}

		balance, err := s.ledger.ClosingBalance(ctx, acc, day)
		if err != nil {
			return err
		}
		if !balance.IsNegative() {
			continue
		}

		if _, err := s.ledger.Append(ctx, entity.LedgerEntry{
			EventID:    entity.EventID("FEE-D" + strconv.Itoa(int(day))),
			AccountID:  acc.ID,
			PostingDay: processingDay,
			ValueDate:  day,
			Amount:     amount.Neg(),
			Origin:     entity.OriginOverdraftFee,
			Memo:       "overdraft fee for day " + strconv.Itoa(int(day)),
		}); err != nil {
			return err
		}
		if err := s.markAssessed(ctx, acc.ID, day); err != nil {
			return err
		}
	}
	return nil
}

// ReverseOnCauseReversal is the acceptance-criterion-6 branch, active only
// under LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal.
//
// Criterion 6 claims that after E9 all balances and fees return to their pre-E7
// values. Under the brief as written that is false: the brief grants an
// assessment primitive and no de-assessment primitive, so the three fees stand
// and the account is permanently 75.10 short of where it would have been.
//
// The criterion is not, however, arithmetically wrong -- it fails on a missing
// rule. Adding that rule does not violate append-only: a fee is reversed by
// appending a contra entry, never by touching the original. This function is
// that rule, so the argument in REJECTED.md can be executed rather than merely
// asserted. It is off by default because defaulting to my own extension would
// quietly overwrite the specification.
//
// A fee is reversed when its day is no longer negative once that fee itself is
// excluded. Excluding it is essential: a fee makes its own day more negative,
// so a fee evaluated inclusively would forever justify its own existence.
//
// Oscillation is impossible by construction, which is why the loop is safe.
// Assessment tests the fee-INCLUSIVE balance and requires it negative; reversal
// tests the fee-EXCLUSIVE balance and requires it non-negative. A day satisfying
// the reversal test has a non-negative balance without its fee, so it would not
// have been assessed in that state. The iteration cap guards against a future
// rule change breaking that property, not against this one.
func (s *service) ReverseOnCauseReversal(
	ctx context.Context, acc entity.Account, processingDay entity.Day,
) error {
	if s.cfg.FeeReversal != config.FeeReversalOnCauseReversal {
		return nil
	}

	for iter := 0; ; iter++ {
		if iter > s.cfg.MaxSweepIterations {
			return apperror.New(apperror.ErrNoConvergence,
				"fee reversal sweep did not converge for "+acc.ID)
		}

		entries, err := s.ledger.Entries(ctx)
		if err != nil {
			return err
		}
		reversed := make(map[int]bool, len(entries))
		for _, e := range entries {
			if e.IsFeeReversal() {
				reversed[e.ReversesSeq] = true
			}
		}

		progressed := false
		for _, charge := range entries {
			if charge.AccountID != acc.ID || !charge.IsFee() || reversed[charge.Seq] {
				continue
			}
			without, err := s.ledger.ClosingBalanceExcluding(ctx, acc, charge.ValueDate, charge.Seq)
			if err != nil {
				return err
			}
			if without.IsNegative() {
				continue
			}
			if _, err := s.ledger.Append(ctx, entity.LedgerEntry{
				EventID:     charge.EventID,
				AccountID:   acc.ID,
				PostingDay:  processingDay,
				ValueDate:   charge.ValueDate,
				Amount:      charge.Amount.Neg(),
				Origin:      entity.OriginFeeReversal,
				Memo:        "reversal of overdraft fee for day " + strconv.Itoa(int(charge.ValueDate)),
				ReversesSeq: charge.Seq,
			}); err != nil {
				return err
			}
			if err := s.repo.Clear(ctx, acc.ID, charge.ValueDate); err != nil {
				return apperror.New(apperror.ErrUnexpected, "clear fee assessment error", err)
			}
			progressed = true
		}

		if !progressed {
			return nil
		}
	}
}
