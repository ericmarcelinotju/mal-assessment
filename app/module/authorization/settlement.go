package authorization

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

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
