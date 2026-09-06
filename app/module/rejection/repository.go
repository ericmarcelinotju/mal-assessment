package rejection

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository stores refused instructions.
//
// Create and Read only. A rejection is a historical fact -- this instruction
// arrived on this day and was refused for this reason -- and editing or
// deleting one would falsify the audit trail just as surely as editing a
// journal entry would.
type Repository interface {
	Create(context.Context, entity.LedgerError) (entity.LedgerError, error)
	Read(context.Context, entity.RejectionFilter) ([]entity.LedgerError, error)
}

type repository struct {
	rejections []entity.LedgerError
}

func NewRepository() Repository {
	return &repository{}
}

func (s *repository) Create(_ context.Context, e entity.LedgerError) (entity.LedgerError, error) {
	s.rejections = append(s.rejections, e)
	return e, nil
}

func (s *repository) Read(
	_ context.Context, filter entity.RejectionFilter,
) ([]entity.LedgerError, error) {
	out := make([]entity.LedgerError, 0, len(s.rejections))
	for _, e := range s.rejections {
		if filter.AccountID != "" && filter.AccountID != e.AccountID {
			continue
		}
		if filter.Day != 0 && filter.Day != e.Day {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
