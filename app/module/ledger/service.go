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
