package interest

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Accrue books the interest for every day up to the processing day, restating
// any earlier day whose closing balance has since changed.
//
// The rule is 0.04% per day on the closing ledger balance, positive balances
// only. The rate is held as the exact rational 4/10000 and applied to the
// balance in one multiplication, so the whole calculation rounds exactly once.
//
// Restatement is the interesting part. E7 arrives on Day 5 back-valued to Day 2,
// which changes the closing balance of Days 2 through 5 after those days
// already accrued. Interest is a function of the value-dated balance, so those
// accruals are now wrong, and nothing has been capitalised yet -- the error is
// still correctable.
//
// The correction is appended, never applied in place. If Day 2 accrued 0.10 and
// should now accrue 0.00, this books a -0.10 adjustment record; the original
// +0.10 stays exactly as it was written. The net accrual for a day is the sum of
// its records, and the trail shows an auditor what the ledger believed and when.
// That is a back-value interest adjustment, which is what a core banking system
// does with a late-arriving entry.
//
// The alternative -- freeze each day's accrual at its own close and let
// back-value entries lie -- is simpler and defensible, and produces AED 0.71
// instead of 0.93 on this stream. It is rejected because it leaves the accrual
// subledger describing balances the ledger no longer holds. See AMBIGUITIES.md.
func (s *service) Accrue(ctx context.Context, acc entity.Account, processingDay entity.Day) error {
	for day := entity.Day(1); day <= processingDay; day++ {
		balance, err := s.ledger.ClosingBalance(ctx, acc, day)
		if err != nil {
			return err
		}

		// Positive balances only. A negative balance accrues nothing; it is
		// priced by the overdraft fee instead, and charging both would be
		// charging twice for one condition.
		target := entity.Zero(acc.Currency)
		if balance.IsPositive() {
			target = balance.MulRatioHalfUp(s.cfg.InterestRateNum, s.cfg.InterestRateDen)
		}

		booked, err := s.NetAccrual(ctx, acc, day)
		if err != nil {
			return err
		}
		delta := target.Sub(booked)
		if delta.IsZero() {
			continue
		}

		if _, err := s.repo.AppendAccrual(ctx, entity.Accrual{
			AccountID:    acc.ID,
			Day:          day,
			BookedOnDay:  processingDay,
			Amount:       delta,
			Basis:        balance,
			IsAdjustment: !booked.IsZero() || day != processingDay,
		}); err != nil {
			return apperror.New(apperror.ErrUnexpected, "append accrual error", err)
		}
	}
	return nil
}

// Capitalise books the accrued interest as a single credit.
//
// The credit is the sum of the rounded daily accruals. This is the direct
// reading of the rule that "the rounded daily accruals must sum exactly to the
// capitalised total", and it is the reason acceptance criterion 8 is refused:
// there is nothing to discard, because the total is defined as the sum rather
// than computed independently and reconciled afterwards.
//
// The distinction is not academic on this stream. ACC-001's exact unrounded
// interest is 0.9180, which would round to 0.92; the rounded dailies sum to
// 0.93. Computing the total independently and discarding the difference would
// open a one-cent break between the accrual subledger and the capitalisation
// entry -- small, permanent, and exactly the kind of break that makes a ledger
// untrustworthy. Capitalising the sum keeps the two sides equal by construction.
//
// The daily accrual is the primitive here and capitalisation is derived from it.
// The opposite arrangement -- treat the exact total as primitive and use
// largest-remainder to force the dailies to sum to it -- also satisfies the rule
// and yields 0.92. See AMBIGUITIES.md for why the daily is the primitive.
func (s *service) Capitalise(ctx context.Context, acc entity.Account, day entity.Day) error {
	accruals, err := s.Accruals(ctx)
	if err != nil {
		return err
	}

	total := entity.Zero(acc.Currency)
	for _, a := range accruals {
		if a.AccountID == acc.ID {
			total = total.Add(a.Amount)
		}
	}
	if total.IsZero() {
		return nil
	}

	_, err = s.ledger.Append(ctx, entity.LedgerEntry{
		EventID:    entity.EventID("INT"),
		AccountID:  acc.ID,
		PostingDay: day,
		ValueDate:  day,
		Amount:     total,
		Origin:     entity.OriginCapitalisation,
		Memo:       "interest capitalisation, sum of daily accruals",
	})
	return err
}
