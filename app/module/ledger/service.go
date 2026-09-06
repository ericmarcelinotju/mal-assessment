// Package ledger is an in-memory, append-only account ledger core.
//
// There is no persistence, no transport and no UI here by design: the service
// is a pure function of the event stream, which is what makes the six-day
// replay reproducible and the acceptance criteria checkable.
package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// Service replays an event stream and reports the resulting position.
type Service interface {
	// Post applies one instruction. It returns the entries it produced, which
	// may be none: a declined authorization and a rejected settlement both
	// produce zero entries and a recorded error.
	Post(context.Context, entity.Event) ([]entity.LedgerEntry, error)
	// CloseDay runs the end-of-day cycle: overdraft assessment, the
	// policy-gated fee reversal sweep, interest accrual and restatement, and on
	// the capitalisation day the single interest credit.
	CloseDay(context.Context, entity.Day) error
	// Replay posts every event in order and closes every day of the window.
	Replay(context.Context, []entity.Event) error
	// Report returns one row per day per account, plus the audit trail.
	Report(context.Context) ([]entity.DayReport, error)
	// Entries, Accruals and Errors expose the append-only log for inspection.
	Entries(context.Context) ([]entity.LedgerEntry, error)
	Accruals(context.Context) ([]entity.Accrual, error)
	Errors(context.Context) ([]entity.LedgerError, error)
	// Config reports the active configuration, so a caller rendering the report
	// can state which fee-reversal policy produced the numbers.
	Config() config.Config
}

type service struct {
	cfg  config.Config
	repo Repository

	accounts map[string]entity.Account
	order    []string

	auths   map[string]*entity.Authorization
	authIDs []string

	// feeDays records the days already charged an overdraft fee, so the "once
	// per day per account" cap is enforced by construction rather than by
	// re-scanning the log.
	feeDays map[feeKey]struct{}
}

type feeKey struct {
	accountID string
	day       entity.Day
}

// NewService builds the ledger service over a repository.
//
// The configuration is passed in rather than read from the environment here, so
// a test selects a fee-reversal policy without mutating process state. The
// accounts are passed in for the same reason: nothing in this package knows
// where they came from.
func NewService(cfg config.Config, repo Repository, accounts ...entity.Account) Service {
	s := &service{
		cfg:      cfg,
		repo:     repo,
		accounts: make(map[string]entity.Account, len(accounts)),
		auths:    make(map[string]*entity.Authorization),
		feeDays:  make(map[feeKey]struct{}),
	}
	for _, a := range accounts {
		s.accounts[a.ID] = a
		s.order = append(s.order, a.ID)
	}
	return s
}

func (s *service) Config() config.Config { return s.cfg }

func (s *service) Entries(ctx context.Context) ([]entity.LedgerEntry, error) {
	res, err := s.repo.Entries(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read ledger entries error", err)
	}
	return res, nil
}

func (s *service) Accruals(ctx context.Context) ([]entity.Accrual, error) {
	res, err := s.repo.Accruals(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read accruals error", err)
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

func (s *service) account(id string) (entity.Account, error) {
	a, ok := s.accounts[id]
	if !ok {
		return entity.Account{}, apperror.New(apperror.ErrUnknownAccount, "unknown account "+id)
	}
	return a, nil
}

// appendEntry is the single place the service writes an entry, so the error
// wrapping is stated once rather than at every call site.
func (s *service) appendEntry(ctx context.Context, e entity.LedgerEntry) (entity.LedgerEntry, error) {
	res, err := s.repo.AppendEntry(ctx, e)
	if err != nil {
		return entity.LedgerEntry{}, apperror.New(apperror.ErrUnexpected, "append ledger entry error", err)
	}
	return res, nil
}

func (s *service) appendAccrual(ctx context.Context, a entity.Accrual) error {
	if _, err := s.repo.AppendAccrual(ctx, a); err != nil {
		return apperror.New(apperror.ErrUnexpected, "append accrual error", err)
	}
	return nil
}
