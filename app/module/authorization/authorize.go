package authorization

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

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
