// Package ledger is an in-memory, append-only account ledger core.
//
// There is no persistence, no transport and no UI here by design: the engine is
// a pure function of the event stream, which is what makes the six-day replay
// reproducible and the acceptance criteria checkable.
package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// Service replays an event stream and reports the resulting position.
type Service interface {
	// Post applies one instruction. It returns the entries it produced, which
	// may be none: a declined authorization and a rejected settlement both
	// produce zero entries and a recorded error.
	Post(entity.Event) ([]entity.LedgerEntry, error)
	// CloseDay runs the end-of-day cycle: overdraft assessment, the
	// policy-gated fee reversal sweep, interest accrual and restatement, and on
	// the capitalisation day the single interest credit.
	CloseDay(entity.Day) error
	// Replay posts every event in order and closes every day of the window.
	Replay([]entity.Event) error
	// Report returns one row per day per account, plus the audit trail.
	Report() []entity.DayReport
	// Entries, Accruals and Errors expose the append-only log for inspection.
	Entries() []entity.LedgerEntry
	Accruals() []entity.Accrual
	Errors() []entity.LedgerError
	// Config returns the active configuration, so the report header can state
	// which fee-reversal policy produced the numbers.
	Config() config.Config
}

type service struct {
	cfg      config.Config
	accounts map[string]entity.Account
	order    []string
	log      *log
	auths    map[string]*entity.Authorization
	authIDs  []string
	// feeDays records the days already charged an overdraft fee, so the "once
	// per day per account" cap is enforced by construction rather than by
	// re-scanning the log.
	feeDays map[feeKey]struct{}
}

type feeKey struct {
	accountID string
	day       entity.Day
}

// New builds an engine. The configuration is passed in rather than read from
// the environment here, so a test selects a fee-reversal policy without
// mutating process state.
func New(cfg config.Config, accounts ...entity.Account) Service {
	s := &service{
		cfg:      cfg,
		accounts: make(map[string]entity.Account, len(accounts)),
		log:      newLog(),
		auths:    make(map[string]*entity.Authorization),
		feeDays:  make(map[feeKey]struct{}),
	}
	for _, a := range accounts {
		s.accounts[a.ID] = a
		s.order = append(s.order, a.ID)
	}
	return s
}

func (s *service) Config() config.Config         { return s.cfg }
func (s *service) Entries() []entity.LedgerEntry { return s.log.Entries() }
func (s *service) Accruals() []entity.Accrual    { return s.log.Accruals() }
func (s *service) Errors() []entity.LedgerError  { return s.log.Errors() }

func (s *service) account(id string) (entity.Account, error) {
	a, ok := s.accounts[id]
	if !ok {
		return entity.Account{}, apperrUnknownAccount(id)
	}
	return a, nil
}
