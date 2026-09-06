package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// settle consumes an authorization and books the settled amount.
//
// A settlement referencing an authorization the ledger has never seen is
// rejected and no funds move (acceptance criterion 4). The alternative -- the
// card-network "force post", where an unmatched settlement is booked and
// flagged for investigation -- is a real practice, and AMBIGUITIES.md argues it
// properly and gives the full numeric effect of choosing it. It is not what
// this engine does: an in-memory core with no upstream network to reconcile
// against has no basis on which to fabricate a debit, and rejecting is the
// recoverable choice of the two. A rejected settlement can be re-presented once
// its authorization arrives; a wrongly booked debit has already left.
//
// A settlement is not subjected to the available-balance test. The funds were
// reserved when the authorization was approved, and reneging on a promise
// already made to a merchant is not a decision the ledger gets to take at
// settlement time.
func (s *service) settle(acc entity.Account, ev entity.Event) ([]entity.LedgerEntry, error) {
	auth, ok := s.auths[ev.AuthID]
	if !ok {
		return nil, s.reject(ev, apperror.ErrOrphanSettlement,
			"settlement for "+ev.AuthID+" has no preceding authorization; rejected, no funds moved")
	}
	if auth.AccountID != acc.ID {
		return nil, s.reject(ev, apperror.ErrInvalidParameter,
			"authorization "+ev.AuthID+" belongs to "+auth.AccountID)
	}
	if auth.State != entity.AuthApproved {
		return nil, s.reject(ev, apperror.ErrAuthNotActive,
			"authorization "+ev.AuthID+" is "+string(auth.State)+", cannot settle")
	}

	entries, err := s.postAmount(acc, ev, ev.Amount.Neg(), entity.OriginSettlement)
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
	return entries, nil
}

// reverse books a contra entry against an earlier event.
//
// The reversed entry is not touched. The ledger is append-only, so undoing E7
// means appending +620.00 at E7's own value date and leaving both records
// standing. The value date matters: reversing at the original value date
// restores the balance of every affected day, whereas reversing at today's date
// would leave the historical days permanently wrong while making today's total
// look right.
func (s *service) reverse(acc entity.Account, ev entity.Event) ([]entity.LedgerEntry, error) {
	var targets []entity.LedgerEntry
	for _, e := range s.log.entries {
		if e.EventID == ev.ReversesEventID && e.AccountID == acc.ID {
			targets = append(targets, e)
		}
	}
	if len(targets) == 0 {
		return nil, s.reject(ev, apperror.ErrUnknownEventRef,
			"reversal references "+string(ev.ReversesEventID)+", which has no entries on "+acc.ID)
	}
	for _, e := range s.log.entries {
		if e.Origin == entity.OriginReversal && e.ReversesSeq == targets[0].Seq {
			return nil, s.reject(ev, apperror.ErrAlreadyReversed,
				string(ev.ReversesEventID)+" is already reversed")
		}
	}

	out := make([]entity.LedgerEntry, 0, len(targets))
	for _, t := range targets {
		out = append(out, s.log.append(entity.LedgerEntry{
			EventID:     ev.ID,
			AccountID:   acc.ID,
			PostingDay:  ev.PostingDay,
			ValueDate:   t.ValueDate,
			Amount:      t.Amount.Neg(),
			Origin:      entity.OriginReversal,
			Memo:        "reversal of " + string(t.EventID),
			ReversesSeq: t.Seq,
		}))
	}
	return out, nil
}
