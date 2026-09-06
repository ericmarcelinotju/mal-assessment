package fee

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository records which account-days have already been charged an overdraft
// fee, so the "at most once per day per account" cap is enforced by stored fact
// rather than by re-deriving it from the journal on every sweep.
//
// Clear exists because the cap only means "ever" while fees are irreversible.
// Under the fee-reversal policy a day whose fee was undone becomes chargeable
// again, and the cap relaxes to once per overdraft episode. See AMBIGUITIES.md.
type Repository interface {
	MarkAssessed(context.Context, string, entity.Day) error
	IsAssessed(context.Context, string, entity.Day) (bool, error)
	Clear(context.Context, string, entity.Day) error
}

type key struct {
	accountID string
	day       entity.Day
}

type repository struct {
	assessed map[key]struct{}
}

func NewRepository() Repository {
	return &repository{assessed: make(map[key]struct{})}
}

func (s *repository) MarkAssessed(_ context.Context, accountID string, day entity.Day) error {
	s.assessed[key{accountID, day}] = struct{}{}
	return nil
}

func (s *repository) IsAssessed(_ context.Context, accountID string, day entity.Day) (bool, error) {
	_, ok := s.assessed[key{accountID, day}]
	return ok, nil
}

func (s *repository) Clear(_ context.Context, accountID string, day entity.Day) error {
	delete(s.assessed, key{accountID, day})
	return nil
}
