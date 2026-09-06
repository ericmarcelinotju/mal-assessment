package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// reject records a refused instruction on the log and returns the error. Every
// rejection is both surfaced to the caller and retained for the day's report:
// "nothing happened" is a fact an operator needs stated, not an absence.
func (s *service) reject(
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
