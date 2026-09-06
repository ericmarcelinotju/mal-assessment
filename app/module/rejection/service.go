// Package rejection owns the record of instructions the ledger refused.
//
// It is separate from the journal because a rejection is not an entry: nothing
// moved. Keeping the two apart means the journal repository is CRUD over
// postings alone, and it makes "no funds left the account" a positive, stored
// fact rather than the absence of a row somewhere else.
package rejection

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

type Service interface {
	Create(context.Context, entity.Event, apperror.ErrorCode, string) error
	Read(context.Context, entity.RejectionFilter) ([]entity.LedgerError, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// Create records a refused instruction and returns the coded error, so a caller
// can record and return in one statement. Every rejection is both surfaced to
// the caller and retained for the day's report: "nothing happened" is a fact an
// operator needs stated, not an absence.
func (s *service) Create(
	ctx context.Context, ev entity.Event, code apperror.ErrorCode, msg string,
) error {
	if _, err := s.repo.Create(ctx, entity.LedgerError{
		Day:       ev.PostingDay,
		EventID:   ev.ID,
		AccountID: ev.AccountID,
		Code:      string(code),
		Message:   msg,
	}); err != nil {
		return apperror.New(apperror.ErrUnexpected, "create rejection error", err)
	}
	return apperror.New(code, msg)
}

func (s *service) Read(
	ctx context.Context, filter entity.RejectionFilter,
) ([]entity.LedgerError, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read rejection error", err)
	}
	return res, nil
}
