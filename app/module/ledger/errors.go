package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

func apperrUnknownAccount(id string) error {
	return apperror.New(apperror.ErrUnknownAccount, "unknown account "+id)
}

// reject records a refused instruction on the log and returns the error. Every
// rejection is both surfaced to the caller and retained for the day's report:
// "nothing happened" is a fact an operator needs stated, not an absence.
func (s *service) reject(ev entity.Event, code apperror.ErrorCode, msg string) error {
	err := apperror.New(code, msg)
	s.log.appendError(entity.LedgerError{
		Day:       ev.PostingDay,
		EventID:   ev.ID,
		AccountID: ev.AccountID,
		Code:      string(code),
		Message:   msg,
	})
	return err
}
