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

type Service interface {
	// Accrue books interest for every day up to the processing day, restating
	// any earlier day whose closing balance has since changed.
	Accrue(context.Context, entity.Account, entity.Day) error
	// Capitalise books the accrued interest as a single credit.
	Capitalise(context.Context, entity.Account, entity.Day) error
	// NetAccrual is the accrual currently standing for one account-day.
	NetAccrual(context.Context, entity.Account, entity.Day) (entity.Money, error)
	Accruals(context.Context) ([]entity.Accrual, error)
}

type service struct {
	cfg    config.Config
	repo   Repository
	ledger ledger.Service
}

func NewService(cfg config.Config, repo Repository, ledgerSvc ledger.Service) Service {
	return &service{cfg: cfg, repo: repo, ledger: ledgerSvc}
}

func (s *service) Accruals(ctx context.Context) ([]entity.Accrual, error) {
	res, err := s.repo.Accruals(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read accruals error", err)
	}
	return res, nil
}

// NetAccrual is the accrual currently standing for one account-day: the sum of
// its records, original plus every adjustment.
func (s *service) NetAccrual(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	accruals, err := s.Accruals(ctx)
	if err != nil {
		return entity.Money{}, err
	}
	total := entity.Zero(acc.Currency)
	for _, a := range accruals {
		if a.AccountID == acc.ID && a.Day == day {
			total = total.Add(a.Amount)
		}
	}
	return total, nil
}
