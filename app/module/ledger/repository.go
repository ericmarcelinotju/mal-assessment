package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository is the append-only journal.
//
// Note what is missing. There is no Update, no Delete and no method that takes
// a sequence number and writes to it. Append-only is enforced by the shape of
// this interface, not by a convention someone has to remember: the operation
// that would rewrite history does not exist to be called.
type Repository interface {
	AppendEntry(context.Context, entity.LedgerEntry) (entity.LedgerEntry, error)
	Entries(context.Context) ([]entity.LedgerEntry, error)
	AppendError(context.Context, entity.LedgerError) error
	Errors(context.Context) ([]entity.LedgerError, error)
}

// repository is the in-memory journal. Its slices are never handed out by
// reference -- the readers return copies -- so a caller cannot rewrite history
// through a slice header it was given.
type repository struct {
	entries []entity.LedgerEntry
	errors  []entity.LedgerError
	seq     int
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

func (s *repository) Entries(_ context.Context) ([]entity.LedgerEntry, error) {
	out := make([]entity.LedgerEntry, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

func (s *repository) AppendError(_ context.Context, e entity.LedgerError) error {
	s.errors = append(s.errors, e)
	return nil
}

func (s *repository) Errors(_ context.Context) ([]entity.LedgerError, error) {
	out := make([]entity.LedgerError, len(s.errors))
	copy(out, s.errors)
	return out, nil
}
