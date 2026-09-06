package interest

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository is the accrual subledger.
//
// Create and Read only, for the same reason as the journal: it is append-only.
// When a back-value posting invalidates a day's accrual, the fix is a further
// record carrying the difference, not an edit of the original. The day 2 trail
// reading +0.10 / -0.10 / +0.09 is the whole point, and an Update would erase
// exactly the history it exists to show.
type Repository interface {
	Create(context.Context, entity.Accrual) (entity.Accrual, error)
	Read(context.Context, entity.AccrualFilter) ([]entity.Accrual, error)
}

type repository struct {
	accruals []entity.Accrual
	seq      int
}

func NewRepository() Repository {
	return &repository{}
}

func (s *repository) Create(_ context.Context, a entity.Accrual) (entity.Accrual, error) {
	s.seq++
	a.Seq = s.seq
	s.accruals = append(s.accruals, a)
	return a, nil
}

func (s *repository) Read(
	_ context.Context, filter entity.AccrualFilter,
) ([]entity.Accrual, error) {
	out := make([]entity.Accrual, 0, len(s.accruals))
	for _, a := range s.accruals {
		if filter.AccountID != "" && filter.AccountID != a.AccountID {
			continue
		}
		if filter.Day != 0 && filter.Day != a.Day {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}
