// Package fee owns the overdraft fee: when it is assessed, and -- under the
// criterion-6 policy -- when it is undone.
//
// It depends on the ledger module for balances and for booking the fee entry,
// and owns only the record of which days have been charged.
package fee

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

type Service interface {
	// Assess charges any day up to the processing day whose closing balance is
	// negative and which has not been charged before.
	Assess(context.Context, entity.Account, entity.Day) error
	// ReverseOnCauseReversal undoes fees whose day is no longer negative, if the
	// configured policy allows it. A no-op under the default policy.
	ReverseOnCauseReversal(context.Context, entity.Account, entity.Day) error
}

type service struct {
	cfg    config.Config
	repo   Repository
	ledger ledger.Service
}

func NewService(cfg config.Config, repo Repository, ledgerSvc ledger.Service) Service {
	return &service{cfg: cfg, repo: repo, ledger: ledgerSvc}
}

// amount returns the overdraft fee denominated in the account's own currency.
//
// The brief states the fee as "AED 25.00" while ACC-002 is a BHD account. An
// AED-denominated entry cannot be booked to a BHD account without an exchange
// rate, and the brief supplies none, so the fee is 25 units of the account's
// currency. ACC-002 never goes negative, so this is not exercised by the
// canonical stream -- but the module has to have an answer, and silently
// booking AED into a BHD account would be the worse one. See NUMBERS.md.
func (s *service) amount(acc entity.Account) entity.Money {
	return entity.NewMoney(
		decimal.NewFromInt(s.cfg.OverdraftFeeMinor).Div(decimal.NewFromInt(100)),
		acc.Currency,
	)
}

func (s *service) markAssessed(ctx context.Context, accountID string, day entity.Day) error {
	if err := s.repo.MarkAssessed(ctx, accountID, day); err != nil {
		return apperror.New(apperror.ErrUnexpected, "mark fee assessed error", err)
	}
	return nil
}
