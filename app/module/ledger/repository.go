package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository is the append-only journal.
//
// Create and Read, and deliberately nothing else. There is no Update and no
// Delete because the journal is append-only: a correction is a new entry with
// the opposite sign, never an edit. That property is enforced by the shape of
// this interface rather than by a convention someone has to remember -- the
// operation that would rewrite history does not exist to be called.
//
// This is the one place the CRUD naming is deliberately incomplete, and the
// incompleteness is the design.
type Repository interface {
	Create(context.Context, entity.LedgerEntry) (entity.LedgerEntry, error)
	Read(context.Context, entity.LedgerEntryFilter) ([]entity.LedgerEntry, error)
}

// repository is the in-memory journal. Its slice is never handed out by
// reference -- Read builds a new one -- so a caller cannot rewrite history
// through a slice header it was given.
type repository struct {
	entries []entity.LedgerEntry
	seq     int
}

func NewRepository() Repository {
	return &repository{}
}

// Create stamps the entry with the next sequence number and appends it. The
// caller cannot influence Seq: it is assigned here and returned on the copy,
// which is what makes the ordering trustworthy.
func (s *repository) Create(_ context.Context, e entity.LedgerEntry) (entity.LedgerEntry, error) {
	s.seq++
	e.Seq = s.seq
	s.entries = append(s.entries, e)
	return e, nil
}

// Read applies the filter in the store rather than handing the whole journal
// back for the caller to loop over. MaxValueDate is the important one: "every
// entry with value_date <= that day" is the brief's own definition of a closing
// balance, so it is expressed once, here.
func (s *repository) Read(
	_ context.Context, filter entity.LedgerEntryFilter,
) ([]entity.LedgerEntry, error) {
	out := make([]entity.LedgerEntry, 0, len(s.entries))
	for _, e := range s.entries {
		if filter.AccountID != "" && filter.AccountID != e.AccountID {
			continue
		}
		if filter.MaxValueDate != 0 && e.ValueDate > filter.MaxValueDate {
			continue
		}
		if filter.ValueDate != 0 && e.ValueDate != filter.ValueDate {
			continue
		}
		if filter.ExcludeSeq != 0 && e.Seq == filter.ExcludeSeq {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
