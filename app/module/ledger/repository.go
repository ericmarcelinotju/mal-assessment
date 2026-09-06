package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository is the append-only store behind the ledger.
//
// It is deliberately an interface with an in-memory implementation rather than
// a bare struct. The brief forbids persistence, but it does not forbid drawing
// the seam where persistence would go: the service depends on this contract and
// not on a slice, so the storage can be swapped without the domain noticing,
// and the service can be tested against a mock.
//
// Note what is missing. There is no Update, no Delete and no method that takes
// a sequence number and writes to it. Append-only is enforced by the shape of
// this interface, not by a convention someone has to remember: the operation
// that would rewrite history does not exist to be called.
type Repository interface {
	AppendEntry(context.Context, entity.LedgerEntry) (entity.LedgerEntry, error)
	AppendAccrual(context.Context, entity.Accrual) (entity.Accrual, error)
	AppendError(context.Context, entity.LedgerError) error
	Entries(context.Context) ([]entity.LedgerEntry, error)
	Accruals(context.Context) ([]entity.Accrual, error)
	Errors(context.Context) ([]entity.LedgerError, error)
}

// repository is the in-memory store. Its slices are never handed out by
// reference -- the readers return copies -- so a caller cannot rewrite history
// through a slice header it was given.
type repository struct {
	entries  []entity.LedgerEntry
	accruals []entity.Accrual
	errors   []entity.LedgerError

	// seq is a single counter shared by entries and accruals, so the sequence
	// numbers form one total order over the whole log rather than two
	// independent orderings that cannot be interleaved after the fact.
	seq int
}

func NewRepository() Repository {
	return &repository{}
}

// AppendEntry stamps the entry with the next sequence number and appends it.
// The caller cannot influence Seq: it is assigned here and returned on the
// copy, which is what makes the ordering trustworthy.
func (s *repository) AppendEntry(_ context.Context, e entity.LedgerEntry) (entity.LedgerEntry, error) {
	s.seq++
	e.Seq = s.seq
	s.entries = append(s.entries, e)
	return e, nil
}

func (s *repository) AppendAccrual(_ context.Context, a entity.Accrual) (entity.Accrual, error) {
	s.seq++
	a.Seq = s.seq
	s.accruals = append(s.accruals, a)
	return a, nil
}

func (s *repository) AppendError(_ context.Context, e entity.LedgerError) error {
	s.errors = append(s.errors, e)
	return nil
}

func (s *repository) Entries(_ context.Context) ([]entity.LedgerEntry, error) {
	out := make([]entity.LedgerEntry, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

func (s *repository) Accruals(_ context.Context) ([]entity.Accrual, error) {
	out := make([]entity.Accrual, len(s.accruals))
	copy(out, s.accruals)
	return out, nil
}

func (s *repository) Errors(_ context.Context) ([]entity.LedgerError, error) {
	out := make([]entity.LedgerError, len(s.errors))
	copy(out, s.errors)
	return out, nil
}
