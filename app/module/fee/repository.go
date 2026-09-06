package fee

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository records which account-days have already been charged an overdraft
// fee, so the "at most once per day per account" cap is a stored row rather
// than a map hidden inside a service.
//
// Delete is real here, and it is the criterion-6 policy in one method. The cap
// only means "ever" while fees are irreversible; under the fee-reversal policy
// a day whose fee was undone becomes chargeable again, and deleting the
// assessment is how that is expressed. See AMBIGUITIES.md.
//
// There is no Update: an assessment either stands or it does not, and there is
// nothing about one to amend.
type Repository interface {
	Create(context.Context, entity.FeeAssessment) (entity.FeeAssessment, error)
	Read(context.Context, entity.FeeAssessmentFilter) ([]entity.FeeAssessment, error)
	Delete(context.Context, string) error
}

type repository struct {
	assessments map[string]entity.FeeAssessment
	order       []string
}

func NewRepository() Repository {
	return &repository{assessments: make(map[string]entity.FeeAssessment)}
}

func (s *repository) Create(
	_ context.Context, a entity.FeeAssessment,
) (entity.FeeAssessment, error) {
	if _, exists := s.assessments[a.ID()]; exists {
		return entity.FeeAssessment{}, ErrAlreadyExists
	}
	s.assessments[a.ID()] = a
	s.order = append(s.order, a.ID())
	return a, nil
}

func (s *repository) Read(
	_ context.Context, filter entity.FeeAssessmentFilter,
) ([]entity.FeeAssessment, error) {
	out := make([]entity.FeeAssessment, 0, len(s.order))
	for _, id := range s.order {
		a, ok := s.assessments[id]
		if !ok {
			continue
		}
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

func (s *repository) Delete(_ context.Context, id string) error {
	if _, exists := s.assessments[id]; !exists {
		return ErrNotFound
	}
	delete(s.assessments, id)
	for i, existing := range s.order {
		if existing == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}
