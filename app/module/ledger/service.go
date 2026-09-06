// Package ledger owns the append-only journal: the entries themselves and the
// value-dated balances derived from them.
//
// It is the base module. It knows nothing about accounts, holds, fees or
// interest -- those modules depend on this one, never the other way round, so
// the dependency graph stays a DAG and the journal cannot be made to care why
// an entry exists.
package ledger

import (
	"context"
	"strconv"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/rejection"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Service is Create and Read over the journal, plus the domain operations built
// from them.
//
// There is no Update and no Delete, matching the repository: the journal is
// append-only and a correction is a new entry, never an edit. Reverse is the
// closest thing to a delete this module offers, and it appends.
type Service interface {
	// Create books a single entry the caller has already shaped. Fee and
	// interest use it; instruction handling goes through Post.
	Create(context.Context, entity.LedgerEntry) (entity.LedgerEntry, error)
	// Read returns the entries matching a filter.
	Read(context.Context, entity.LedgerEntryFilter) ([]entity.LedgerEntry, error)

	// Post books a signed amount, splitting it into instalments when the event
	// asks for them.
	Post(context.Context, entity.Account, entity.Event, entity.Money, entity.EntryOrigin) ([]entity.LedgerEntry, error)
	// Reverse books contra entries against every entry of an earlier event.
	Reverse(context.Context, entity.Account, entity.Event) ([]entity.LedgerEntry, error)

	// ClosingBalance is the closing ledger balance for a day: opening balance
	// plus every entry with value_date <= day.
	ClosingBalance(context.Context, entity.Account, entity.Day) (entity.Money, error)
	// ClosingBalanceExcluding is the same figure with one entry left out.
	ClosingBalanceExcluding(context.Context, entity.Account, entity.Day, int) (entity.Money, error)
}

type service struct {
	repo       Repository
	rejections rejection.Service
}

func NewService(repo Repository, rejectionSvc rejection.Service) Service {
	return &service{repo: repo, rejections: rejectionSvc}
}

func (s *service) Create(ctx context.Context, payload entity.LedgerEntry) (entity.LedgerEntry, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.LedgerEntry{}, apperror.New(apperror.ErrUnexpected, "create ledger entry error", err)
	}
	return res, nil
}

func (s *service) Read(
	ctx context.Context, filter entity.LedgerEntryFilter,
) ([]entity.LedgerEntry, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read ledger entry error", err)
	}
	return res, nil
}

// Post books a signed amount, splitting it into instalments when the event asks
// for them.
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
			return nil, s.rejections.Create(ctx, ev, apperror.ErrInvalidParameter, err.Error())
		}
		parts = split
	}

	out := make([]entity.LedgerEntry, 0, len(parts))
	for i, p := range parts {
		memo := string(ev.Type)
		if len(parts) > 1 {
			memo = memo + " instalment " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(parts))
		}
		entry, err := s.Create(ctx, entity.LedgerEntry{
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
	all, err := s.Read(ctx, entity.LedgerEntryFilter{AccountID: acc.ID})
	if err != nil {
		return nil, err
	}

	var targets []entity.LedgerEntry
	for _, e := range all {
		if e.EventID == ev.ReversesEventID {
			targets = append(targets, e)
		}
	}
	if len(targets) == 0 {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrUnknownEventRef,
			"reversal references "+string(ev.ReversesEventID)+", which has no entries on "+acc.ID)
	}
	for _, e := range all {
		if e.Origin == entity.OriginReversal && e.ReversesSeq == targets[0].Seq {
			return nil, s.rejections.Create(ctx, ev, apperror.ErrAlreadyReversed,
				string(ev.ReversesEventID)+" is already reversed")
		}
	}

	out := make([]entity.LedgerEntry, 0, len(targets))
	for _, t := range targets {
		entry, err := s.Create(ctx, entity.LedgerEntry{
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

// ClosingBalance is the closing ledger balance for a day: the opening balance
// plus every entry with value_date <= day.
//
// This is the definition the brief gives in parentheses, and it is the reason
// criterion 2 cannot be satisfied. A back-value entry does not land on one day;
// it is a member of the value_date <= d set for its own day and for every later
// day, so it depresses all of them at once.
func (s *service) ClosingBalance(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	return s.sum(ctx, acc, entity.LedgerEntryFilter{
		AccountID: acc.ID, MaxValueDate: day,
	})
}

// ClosingBalanceExcluding is the closing balance with one entry left out.
// The fee reversal sweep needs it: the question "is this day still overdrawn?"
// has to be asked without the fee that is itself under consideration, or the
// fee would forever justify its own existence.
func (s *service) ClosingBalanceExcluding(
	ctx context.Context, acc entity.Account, day entity.Day, seq int,
) (entity.Money, error) {
	return s.sum(ctx, acc, entity.LedgerEntryFilter{
		AccountID: acc.ID, MaxValueDate: day, ExcludeSeq: seq,
	})
}

// sum is the opening balance plus every entry the filter selects. Every balance
// in this module goes through it, which keeps the variants the rules demand
// from drifting apart.
func (s *service) sum(
	ctx context.Context, acc entity.Account, filter entity.LedgerEntryFilter,
) (entity.Money, error) {
	entries, err := s.Read(ctx, filter)
	if err != nil {
		return entity.Money{}, err
	}
	total := acc.Opening
	for _, e := range entries {
		next, err := total.Add(e.Amount)
		if err != nil {
			return entity.Money{}, err
		}
		total = next
	}
	return total, nil
}
