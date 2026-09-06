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
	"github.com/ericmarcelinotju/mal-assessment/app/module/rejection"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Sentinel errors returned by the repository.
var (
	ErrNotFound      = apperror.New(apperror.ErrUnknownEventRef, "authorization not found")
	ErrAlreadyExists = apperror.New(apperror.ErrDuplicateAuthID, "authorization already exists")
)

type Service interface {
	Create(context.Context, entity.Authorization) (entity.Authorization, error)
	Read(context.Context, entity.AuthorizationFilter) ([]entity.Authorization, error)
	Update(context.Context, entity.Authorization) (entity.Authorization, error)
	Delete(context.Context, string) error

	// Authorize applies the availability test and records the outcome.
	Authorize(context.Context, entity.Account, entity.Event) error
	// Settle consumes an approved hold and books the settled amount.
	Settle(context.Context, entity.Account, entity.Event) ([]entity.LedgerEntry, error)
	// ActiveHolds sums the holds outstanding for an account as at a day.
	ActiveHolds(context.Context, entity.Account, entity.Day) (entity.Money, error)
	// AvailableBalance is ledger balance minus active holds.
	AvailableBalance(context.Context, entity.Account, entity.Day) (entity.Money, error)
}

type service struct {
	repo       Repository
	ledger     ledger.Service
	rejections rejection.Service
}

func NewService(repo Repository, ledgerSvc ledger.Service, rejectionSvc rejection.Service) Service {
	return &service{repo: repo, ledger: ledgerSvc, rejections: rejectionSvc}
}

func (s *service) Create(
	ctx context.Context, payload entity.Authorization,
) (entity.Authorization, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.Authorization{}, apperror.New(apperror.ErrUnexpected,
			"create authorization error", err)
	}
	return res, nil
}

func (s *service) Read(
	ctx context.Context, filter entity.AuthorizationFilter,
) ([]entity.Authorization, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read authorization error", err)
	}
	return res, nil
}

func (s *service) Update(
	ctx context.Context, payload entity.Authorization,
) (entity.Authorization, error) {
	res, err := s.repo.Update(ctx, payload)
	if err != nil {
		return entity.Authorization{}, apperror.New(apperror.ErrUnexpected,
			"update authorization error", err)
	}
	return res, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.New(apperror.ErrUnexpected, "delete authorization error", err)
	}
	return nil
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
	existing, err := s.Read(ctx, entity.AuthorizationFilter{AuthID: ev.AuthID})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return s.rejections.Create(ctx, ev, apperror.ErrDuplicateAuthID,
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

	if _, err := s.Create(ctx, auth); err != nil {
		return err
	}
	if auth.State == entity.AuthDeclined {
		return s.rejections.Create(ctx, ev, apperror.ErrInsufficientFunds,
			"authorization "+ev.AuthID+" declined: "+auth.DeclineNote)
	}
	return nil
}

// Settle consumes an approved hold and books the settled amount.
//
// A settlement referencing an authorization the ledger has never seen is
// rejected and no funds move (acceptance criterion 4). The alternative -- the
// card-network force post, where an unmatched settlement is booked and flagged
// for investigation -- is a real practice, and AMBIGUITIES.md argues it
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
	found, err := s.Read(ctx, entity.AuthorizationFilter{AuthID: ev.AuthID})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrOrphanSettlement,
			"settlement for "+ev.AuthID+" has no preceding authorization; rejected, no funds moved")
	}
	auth := found[0]

	if auth.AccountID != acc.ID {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrInvalidParameter,
			"authorization "+ev.AuthID+" belongs to "+auth.AccountID)
	}
	if auth.State != entity.AuthApproved {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrAuthNotActive,
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
	if _, err := s.Update(ctx, auth); err != nil {
		return nil, err
	}
	return entries, nil
}

// ActiveHolds sums the holds outstanding as at a day. A hold counts from its
// value date until the day it is settled; a declined authorization never counts.
func (s *service) ActiveHolds(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	auths, err := s.Read(ctx, entity.AuthorizationFilter{AccountID: acc.ID})
	if err != nil {
		return entity.Money{}, err
	}
	total := entity.Zero(acc.Currency)
	for _, a := range auths {
		if a.ValueDate > day {
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
