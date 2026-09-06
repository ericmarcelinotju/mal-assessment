// Package interest owns the accrual subledger: the daily accrual, the
// restatement of a day whose balance later changed, and the single
// capitalisation credit at the end of the window.
//
// It depends on the ledger module for balances and for booking the
// capitalisation entry, and owns the accrual records themselves.
package interest

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// Service is Create and Read over the accrual subledger, plus the domain
// operations built from them.
//
// There is no Update and no Delete, matching the repository: the subledger is
// append-only, and a day whose accrual turns out wrong gets a further record
// carrying the difference rather than an edit of the original.
type Service interface {
	Create(context.Context, entity.Accrual) (entity.Accrual, error)
	Read(context.Context, entity.AccrualFilter) ([]entity.Accrual, error)

	// Accrue books interest for every day up to the processing day, restating
	// any earlier day whose closing balance has since changed.
	Accrue(context.Context, entity.Account, entity.Day) error
	// Capitalise books the accrued interest as a single credit.
	Capitalise(context.Context, entity.Account, entity.Day) error
	// NetAccrual is the accrual currently standing for one account-day.
	NetAccrual(context.Context, entity.Account, entity.Day) (entity.Money, error)
}

type service struct {
	cfg    config.Config
	repo   Repository
	ledger ledger.Service
}

func NewService(cfg config.Config, repo Repository, ledgerSvc ledger.Service) Service {
	return &service{cfg: cfg, repo: repo, ledger: ledgerSvc}
}

func (s *service) Create(ctx context.Context, payload entity.Accrual) (entity.Accrual, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.Accrual{}, apperror.New(apperror.ErrUnexpected, "create accrual error", err)
	}
	return res, nil
}

func (s *service) Read(
	ctx context.Context, filter entity.AccrualFilter,
) ([]entity.Accrual, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read accrual error", err)
	}
	return res, nil
}

// NetAccrual is the accrual currently standing for one account-day: the sum of
// its records, original plus every adjustment.
func (s *service) NetAccrual(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	accruals, err := s.Read(ctx, entity.AccrualFilter{AccountID: acc.ID, Day: day})
	if err != nil {
		return entity.Money{}, err
	}
	total := entity.Zero(acc.Currency)
	for _, a := range accruals {
		next, err := total.Add(a.Amount)
		if err != nil {
			return entity.Money{}, err
		}
		total = next
	}
	return total, nil
}

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
			target, err = balance.MulRatioHalfUp(s.cfg.InterestRateNum, s.cfg.InterestRateDen)
			if err != nil {
				return err
			}
		}

		booked, err := s.NetAccrual(ctx, acc, day)
		if err != nil {
			return err
		}
		delta, err := target.Sub(booked)
		if err != nil {
			return err
		}
		if delta.IsZero() {
			continue
		}

		if _, err := s.Create(ctx, entity.Accrual{
			AccountID:    acc.ID,
			Day:          day,
			BookedOnDay:  processingDay,
			Amount:       delta,
			Basis:        balance,
			IsAdjustment: !booked.IsZero() || day != processingDay,
		}); err != nil {
			return err
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
	accruals, err := s.Read(ctx, entity.AccrualFilter{AccountID: acc.ID})
	if err != nil {
		return err
	}

	total := entity.Zero(acc.Currency)
	for _, a := range accruals {
		next, err := total.Add(a.Amount)
		if err != nil {
			return err
		}
		total = next
	}
	if total.IsZero() {
		return nil
	}

	_, err = s.ledger.Create(ctx, entity.LedgerEntry{
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
