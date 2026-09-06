package interest

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository is the accrual subledger. Like the journal it is append-only:
// there is no way to rewrite an accrual, only to append a correction beside it.
type Repository interface {
	AppendAccrual(context.Context, entity.Accrual) (entity.Accrual, error)
	Accruals(context.Context) ([]entity.Accrual, error)
}

type repository struct {
	accruals []entity.Accrual
	seq      int
}

func NewRepository() Repository {
	return &repository{}
}

func (s *repository) AppendAccrual(_ context.Context, a entity.Accrual) (entity.Accrual, error) {
	s.seq++
	a.Seq = s.seq
	s.accruals = append(s.accruals, a)
	return a, nil
}

func (s *repository) Accruals(_ context.Context) ([]entity.Accrual, error) {
	out := make([]entity.Accrual, len(s.accruals))
	copy(out, s.accruals)
	return out, nil
}
