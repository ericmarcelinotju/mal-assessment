package ledger

import (
	"context"
	"strconv"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Post appends a signed amount, splitting it into instalments when the event
// asks for them.
//
// Note what is absent: no available-balance test. The brief applies that test
// to authorizations only, and rightly so -- a debit that cannot be funded is
// exactly the situation the overdraft fee exists to price. Refusing it here
// would mean the fee rule could never fire at all.
func (s *service) Post(
	ctx context.Context, acc entity.Account, ev entity.Event,
	signed entity.Money, origin entity.EntryOrigin,
) ([]entity.LedgerEntry, error) {
	parts := []entity.Money{signed}
	if ev.Instalments > 1 {
		split, err := entity.SplitInstalments(signed, ev.Instalments)
		if err != nil {
			return nil, s.Reject(ctx, ev, apperror.ErrInvalidParameter, err.Error())
		}
		parts = split
	}

	out := make([]entity.LedgerEntry, 0, len(parts))
	for i, p := range parts {
		memo := string(ev.Type)
		if len(parts) > 1 {
			memo = memo + " instalment " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(parts))
		}
		entry, err := s.Append(ctx, entity.LedgerEntry{
			EventID:    ev.ID,
			AccountID:  acc.ID,
			PostingDay: ev.PostingDay,
			ValueDate:  ev.ValueDate,
			Amount:     p,
			Origin:     origin,
			Memo:       memo,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

// Reverse books a contra entry against every entry of an earlier event.
//
// The reversed entry is not touched. The journal is append-only, so undoing E7
// means appending +620.00 at E7's own value date and leaving both records
// standing. The value date matters: reversing at the original value date
// restores the balance of every affected day, whereas reversing at today's date
// would leave the historical days permanently wrong while making today's total
// look right.
func (s *service) Reverse(
	ctx context.Context, acc entity.Account, ev entity.Event,
) ([]entity.LedgerEntry, error) {
	all, err := s.Entries(ctx)
	if err != nil {
		return nil, err
	}

	var targets []entity.LedgerEntry
	for _, e := range all {
		if e.EventID == ev.ReversesEventID && e.AccountID == acc.ID {
			targets = append(targets, e)
		}
	}
	if len(targets) == 0 {
		return nil, s.Reject(ctx, ev, apperror.ErrUnknownEventRef,
			"reversal references "+string(ev.ReversesEventID)+", which has no entries on "+acc.ID)
	}
	for _, e := range all {
		if e.Origin == entity.OriginReversal && e.ReversesSeq == targets[0].Seq {
			return nil, s.Reject(ctx, ev, apperror.ErrAlreadyReversed,
				string(ev.ReversesEventID)+" is already reversed")
		}
	}

	out := make([]entity.LedgerEntry, 0, len(targets))
	for _, t := range targets {
		entry, err := s.Append(ctx, entity.LedgerEntry{
			EventID:     ev.ID,
			AccountID:   acc.ID,
			PostingDay:  ev.PostingDay,
			ValueDate:   t.ValueDate,
			Amount:      t.Amount.Neg(),
			Origin:      entity.OriginReversal,
			Memo:        "reversal of " + string(t.EventID),
			ReversesSeq: t.Seq,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}
