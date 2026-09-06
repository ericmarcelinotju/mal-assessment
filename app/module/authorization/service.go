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

// Authorize applies the availability test from the brief: an authorization is
// approved only if available balance -- ledger balance minus active holds --
// remains at or above zero after the hold is applied.
//
// The test runs against the balance as it stands when the authorization
// arrives, before that day's close. Auth-B is declined either way on this
// stream (-245.00 before fees, -320.00 after), so the ordering is not
// load-bearing here, but the choice is deliberate: an authorization is answered
// in real time and cannot wait for a day-end batch that has not run yet.
func (s *service) Authorize(ctx context.Context, acc entity.Account, ev entity.Event) error {
	if _, exists, err := s.repo.Get(ctx, ev.AuthID); err != nil {
		return apperror.New(apperror.ErrUnexpected, "read authorization error", err)
	} else if exists {
		return s.ledger.Reject(ctx, ev, apperror.ErrDuplicateAuthID,
			"authorization "+ev.AuthID+" already exists")
	}

	available, err := s.AvailableBalance(ctx, acc, ev.ValueDate)
	if err != nil {
		return err
	}
	after := available.Sub(ev.Amount)

	auth := entity.Authorization{
		AuthID:     ev.AuthID,
		AccountID:  acc.ID,
		EventID:    ev.ID,
		PostingDay: ev.PostingDay,
		ValueDate:  ev.ValueDate,
		Hold:       ev.Amount,
		State:      entity.AuthApproved,
	}

	// "at or above zero after the hold is applied" -- so zero approves and only
	// a strictly negative result declines.
	if after.IsNegative() {
		auth.State = entity.AuthDeclined
		auth.DeclineNote = "available " + available.String() + " - hold " + ev.Amount.String() +
			" = " + after.String() + ", below zero"
	}

	if err := s.repo.Save(ctx, auth); err != nil {
		return apperror.New(apperror.ErrUnexpected, "save authorization error", err)
	}
	if auth.State == entity.AuthDeclined {
		return s.ledger.Reject(ctx, ev, apperror.ErrInsufficientFunds,
			"authorization "+ev.AuthID+" declined: "+auth.DeclineNote)
	}
	return nil
}

// Settle consumes an approved hold and books the settled amount.
//
// A settlement referencing an authorization the ledger has never seen is
// rejected and no funds move (acceptance criterion 4). The alternative -- the
// card-network "force post", where an unmatched settlement is booked and
// flagged for investigation -- is a real practice, and AMBIGUITIES.md argues it
// properly and gives the full numeric effect of choosing it. It is not what
// this module does: an in-memory core with no upstream network to reconcile
// against has no basis on which to fabricate a debit, and rejecting is the
// recoverable choice of the two. A rejected settlement can be re-presented once
// its authorization arrives; a wrongly booked debit has already left.
//
// A settlement is not subjected to the available-balance test. The funds were
// reserved when the authorization was approved, and reneging on a promise
// already made to a merchant is not a decision the ledger gets to take at
// settlement time.
func (s *service) Settle(
	ctx context.Context, acc entity.Account, ev entity.Event,
) ([]entity.LedgerEntry, error) {
	auth, ok, err := s.repo.Get(ctx, ev.AuthID)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read authorization error", err)
	}
	if !ok {
		return nil, s.ledger.Reject(ctx, ev, apperror.ErrOrphanSettlement,
			"settlement for "+ev.AuthID+" has no preceding authorization; rejected, no funds moved")
	}
	if auth.AccountID != acc.ID {
		return nil, s.ledger.Reject(ctx, ev, apperror.ErrInvalidParameter,
			"authorization "+ev.AuthID+" belongs to "+auth.AccountID)
	}
	if auth.State != entity.AuthApproved {
		return nil, s.ledger.Reject(ctx, ev, apperror.ErrAuthNotActive,
			"authorization "+ev.AuthID+" is "+string(auth.State)+", cannot settle")
	}

	entries, err := s.ledger.Post(ctx, acc, ev, ev.Amount.Neg(), entity.OriginSettlement)
	if err != nil {
		return nil, err
	}

	// The hold is released in full, including the unused portion. Auth-A holds
	// 200.00 and settles 185.00; the remaining 15.00 is released rather than
	// retained, because the authorization is closed and there is nothing left
	// for it to secure. See AMBIGUITIES.md on partial release.
	auth.State = entity.AuthSettled
	auth.SettledDay = ev.ValueDate
	auth.SettledAmount = ev.Amount
	if err := s.repo.Save(ctx, auth); err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "save authorization error", err)
	}
	return entries, nil
}
