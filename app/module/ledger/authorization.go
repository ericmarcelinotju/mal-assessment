package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// authorize applies the availability test from the brief: an authorization is
// approved only if available balance -- ledger balance minus active holds --
// remains at or above zero after the hold is applied.
//
// The test runs against the balance as it stands when the authorization
// arrives, before that day's close. Auth-B is declined either way on this
// stream (-245.00 before fees, -320.00 after), so the ordering is not
// load-bearing here, but the choice is deliberate: an authorization is answered
// in real time and cannot wait for a day-end batch that has not run yet.
func (s *service) authorize(acc entity.Account, ev entity.Event) error {
	if _, exists := s.auths[ev.AuthID]; exists {
		return s.reject(ev, apperror.ErrDuplicateAuthID, "authorization "+ev.AuthID+" already exists")
	}

	available := s.availableBalance(acc, ev.ValueDate)
	after := available.Sub(ev.Amount)

	auth := &entity.Authorization{
		AuthID:     ev.AuthID,
		AccountID:  acc.ID,
		EventID:    ev.ID,
		PostingDay: ev.PostingDay,
		ValueDate:  ev.ValueDate,
		Hold:       ev.Amount,
	}
	s.auths[ev.AuthID] = auth
	s.authIDs = append(s.authIDs, ev.AuthID)

	// "at or above zero after the hold is applied" -- so zero approves and only
	// a strictly negative result declines.
	if after.IsNegative() {
		auth.State = entity.AuthDeclined
		auth.DeclineNote = "available " + available.String() + " - hold " + ev.Amount.String() +
			" = " + after.String() + ", below zero"
		return s.reject(ev, apperror.ErrInsufficientFunds,
			"authorization "+ev.AuthID+" declined: "+auth.DeclineNote)
	}

	auth.State = entity.AuthApproved
	return nil
}

// authorizations returns the authorizations in arrival order.
func (s *service) authorizations() []entity.Authorization {
	out := make([]entity.Authorization, 0, len(s.authIDs))
	for _, id := range s.authIDs {
		out = append(out, *s.auths[id])
	}
	return out
}
