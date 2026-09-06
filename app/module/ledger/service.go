// Package ledger owns the append-only journal: the entries themselves, the
// value-dated balances derived from them, and the record of instructions that
// were refused.
//
// It is the base module. It knows nothing about holds, fees or interest -- those
// modules depend on this one, never the other way round, so the dependency
// graph stays a DAG and the journal cannot be made to care why an entry exists.
package ledger

import (
	"context"
	"strconv"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

type Service interface {
	// Post appends a signed amount, splitting it into instalments when the
	// event asks for them.
	Post(context.Context, entity.Account, entity.Event, entity.Money, entity.EntryOrigin) ([]entity.LedgerEntry, error)
	// Append books a single entry the caller has already shaped. Fee and
	// interest use it; instruction handling goes through Post.
	Append(context.Context, entity.LedgerEntry) (entity.LedgerEntry, error)
	// Reverse books contra entries against every entry of an earlier event.
	Reverse(context.Context, entity.Account, entity.Event) ([]entity.LedgerEntry, error)

	// ClosingBalance is the closing ledger balance for a day: opening balance
	// plus every entry with value_date <= day.
	ClosingBalance(context.Context, entity.Account, entity.Day) (entity.Money, error)
	// ClosingBalanceExcluding is the same figure with one entry left out.
	ClosingBalanceExcluding(context.Context, entity.Account, entity.Day, int) (entity.Money, error)

	Entries(context.Context) ([]entity.LedgerEntry, error)
	Errors(context.Context) ([]entity.LedgerError, error)
	// Reject records a refused instruction and returns the coded error.
	Reject(context.Context, entity.Event, apperror.ErrorCode, string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Append(ctx context.Context, e entity.LedgerEntry) (entity.LedgerEntry, error) {
	res, err := s.repo.AppendEntry(ctx, e)
	if err != nil {
		return entity.LedgerEntry{}, apperror.New(apperror.ErrUnexpected, "append ledger entry error", err)
	}
	return res, nil
}

func (s *service) Entries(ctx context.Context) ([]entity.LedgerEntry, error) {
	res, err := s.repo.Entries(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read ledger entries error", err)
	}
	return res, nil
}

func (s *service) Errors(ctx context.Context) ([]entity.LedgerError, error) {
	res, err := s.repo.Errors(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read ledger errors error", err)
	}
	return res, nil
}

// Reject records a refused instruction on the journal and returns the error.
// Every rejection is both surfaced to the caller and retained for the day's
// report: "nothing happened" is a fact an operator needs stated, not an absence.
func (s *service) Reject(
	ctx context.Context, ev entity.Event, code apperror.ErrorCode, msg string,
) error {
	if err := s.repo.AppendError(ctx, entity.LedgerError{
		Day:       ev.PostingDay,
		EventID:   ev.ID,
		AccountID: ev.AccountID,
		Code:      string(code),
		Message:   msg,
	}); err != nil {
		return apperror.New(apperror.ErrUnexpected, "append ledger error record error", err)
	}
	return apperror.New(code, msg)
}

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

// entryFilter selects which entries participate in a balance. Every balance in
// this module is "the sum of the entries that pass a filter", which keeps the
// variants the rules demand from drifting apart.
type entryFilter func(entity.LedgerEntry) bool

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
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day
	})
}

// ClosingBalanceExcluding is the closing balance with one entry left out.
// The fee reversal sweep needs it: the question "is this day still overdrawn?"
// has to be asked without the fee that is itself under consideration, or the
// fee would forever justify its own existence.
func (s *service) ClosingBalanceExcluding(
	ctx context.Context, acc entity.Account, day entity.Day, seq int,
) (entity.Money, error) {
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day && e.Seq != seq
	})
}

func (s *service) balanceWhere(
	ctx context.Context, acc entity.Account, keep entryFilter,
) (entity.Money, error) {
	all, err := s.Entries(ctx)
	if err != nil {
		return entity.Money{}, err
	}
	total := acc.Opening
	for _, e := range all {
		if keep(e) {
			total = total.Add(e.Amount)
		}
	}
	return total, nil
}
