package ledger

import (
	"github.com/shopspring/decimal"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// assessOverdraftFees sweeps every day of the window up to and including the
// processing day, charging the fee to any day whose closing ledger balance is
// negative and which has not been charged before.
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
// simultaneous evaluation. TestFeeCascade covers that gap with a synthetic case.
func (s *service) assessOverdraftFees(acc entity.Account, processingDay entity.Day) error {
	fee := s.feeAmount(acc)
	for day := entity.Day(1); day <= processingDay; day++ {
		if _, charged := s.feeDays[feeKey{acc.ID, day}]; charged {
			continue
		}
		if !s.closingBalance(acc, day).IsNegative() {
			continue
		}
		s.log.append(entity.LedgerEntry{
			EventID:    entity.EventID("FEE-D" + itoa(int(day))),
			AccountID:  acc.ID,
			PostingDay: processingDay,
			ValueDate:  day,
			Amount:     fee.Neg(),
			Origin:     entity.OriginOverdraftFee,
			Memo:       "overdraft fee for day " + itoa(int(day)),
		})
		s.feeDays[feeKey{acc.ID, day}] = struct{}{}
	}
	return nil
}

// feeAmount returns the overdraft fee denominated in the account's own
// currency.
//
// The brief states the fee as "AED 25.00" while ACC-002 is a BHD account. An
// AED-denominated entry cannot be booked to a BHD account without an exchange
// rate, and the brief supplies none, so the fee is treated as 25 units of the
// account's currency. ACC-002 never goes negative, so this choice is not
// exercised by the canonical stream -- but the engine has to have an answer, and
// silently booking AED into a BHD account would be the worse one. See
// NUMBERS.md and AMBIGUITIES.md.
func (s *service) feeAmount(acc entity.Account) entity.Money {
	return entity.NewMoney(decimal.NewFromInt(s.cfg.OverdraftFeeMinor).Div(decimal.NewFromInt(100)), acc.Currency)
}

// reverseFeesOnCauseReversal is the acceptance-criterion-6 branch, active only
// under LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal.
//
// Criterion 6 claims that after E9 all balances and fees return to their
// pre-E7 values. Under the brief as written that is false: the brief grants an
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
// have been assessed in that state. The iteration cap is a guard against a
// future rule change breaking that property, not against this one.
func (s *service) reverseFeesOnCauseReversal(acc entity.Account, processingDay entity.Day) error {
	if s.cfg.FeeReversal != config.FeeReversalOnCauseReversal {
		return nil
	}

	for iter := 0; ; iter++ {
		if iter > s.cfg.MaxSweepIterations {
			return apperror.New(apperror.ErrNoConvergence,
				"fee reversal sweep did not converge for "+acc.ID)
		}
		progressed := false

		for _, fee := range s.log.Entries() {
			if fee.AccountID != acc.ID || !fee.IsFee() || s.isReversed(fee.Seq) {
				continue
			}
			if s.closingBalanceExcludingSeq(acc, fee.ValueDate, fee.Seq).IsNegative() {
				continue
			}
			s.log.append(entity.LedgerEntry{
				EventID:     fee.EventID,
				AccountID:   acc.ID,
				PostingDay:  processingDay,
				ValueDate:   fee.ValueDate,
				Amount:      fee.Amount.Neg(),
				Origin:      entity.OriginFeeReversal,
				Memo:        "reversal of overdraft fee for day " + itoa(int(fee.ValueDate)),
				ReversesSeq: fee.Seq,
			})
			// Under this policy the day becomes chargeable again if it goes
			// negative later: "once per day ever" only makes sense while fees
			// are irreversible. See AMBIGUITIES.md.
			delete(s.feeDays, feeKey{acc.ID, fee.ValueDate})
			progressed = true
		}

		if !progressed {
			return nil
		}
	}
}

func (s *service) isReversed(seq int) bool {
	for _, e := range s.log.entries {
		if e.Origin == entity.OriginFeeReversal && e.ReversesSeq == seq {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
