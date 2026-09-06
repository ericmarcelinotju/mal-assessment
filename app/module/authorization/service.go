// Package authorization owns holds: the availability test that approves or
// declines them, the settlement that consumes them, and the effect they have on
// available balance.
//
// It depends on the ledger module for balances and for booking the settlement
// entry, and the ledger module knows nothing of it. A hold never moves the
// ledger balance -- that is the whole point of a hold, and the substance of
// acceptance criterion 5 -- so this module is the only place holds exist.
package authorization

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

type Service interface {
	// Authorize applies the availability test and records the outcome.
	Authorize(context.Context, entity.Account, entity.Event) error
	// Settle consumes an approved hold and books the settled amount.
	Settle(context.Context, entity.Account, entity.Event) ([]entity.LedgerEntry, error)
	// ActiveHolds sums the holds outstanding for an account as at a day.
	ActiveHolds(context.Context, entity.Account, entity.Day) (entity.Money, error)
	// AvailableBalance is ledger balance minus active holds.
	AvailableBalance(context.Context, entity.Account, entity.Day) (entity.Money, error)
	All(context.Context) ([]entity.Authorization, error)
}

type service struct {
	repo   Repository
	ledger ledger.Service
}

func NewService(repo Repository, ledgerSvc ledger.Service) Service {
	return &service{repo: repo, ledger: ledgerSvc}
}

func (s *service) All(ctx context.Context) ([]entity.Authorization, error) {
	res, err := s.repo.All(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read authorizations error", err)
	}
	return res, nil
}

// ActiveHolds sums the holds outstanding as at a day. A hold counts from its
// value date until the day it is settled; a declined authorization never counts.
func (s *service) ActiveHolds(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	auths, err := s.All(ctx)
	if err != nil {
		return entity.Money{}, err
	}
	total := entity.Zero(acc.Currency)
	for _, a := range auths {
		if a.AccountID != acc.ID || a.ValueDate > day {
			continue
		}
		switch a.State {
		case entity.AuthApproved:
			total = total.Add(a.Hold)
		case entity.AuthSettled:
			// The hold stood until the settlement landed, so it still reduces
			// availability on the days before that.
			if day < a.SettledDay {
				total = total.Add(a.Hold)
			}
		}
	}
	return total, nil
}

// AvailableBalance is the figure the authorization test uses: ledger balance
// minus active holds.
func (s *service) AvailableBalance(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	balance, err := s.ledger.ClosingBalance(ctx, acc, day)
	if err != nil {
		return entity.Money{}, err
	}
	holds, err := s.ActiveHolds(ctx, acc, day)
	if err != nil {
		return entity.Money{}, err
	}
	return balance.Sub(holds), nil
}
