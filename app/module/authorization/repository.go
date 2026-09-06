package authorization

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository stores authorizations, approved and declined alike. A declined
// authorization is kept because the day report has to explain the decline;
// dropping it would leave an unexplained absence.
type Repository interface {
	Save(context.Context, entity.Authorization) error
	Get(context.Context, string) (entity.Authorization, bool, error)
	All(context.Context) ([]entity.Authorization, error)
}

// repository keeps authorizations in memory in arrival order.
type repository struct {
	auths map[string]entity.Authorization
	order []string
}

func NewRepository() Repository {
	return &repository{auths: make(map[string]entity.Authorization)}
}

func (s *repository) Save(_ context.Context, a entity.Authorization) error {
	if _, exists := s.auths[a.AuthID]; !exists {
		s.order = append(s.order, a.AuthID)
	}
	s.auths[a.AuthID] = a
	return nil
}

func (s *repository) Get(_ context.Context, id string) (entity.Authorization, bool, error) {
	a, ok := s.auths[id]
	return a, ok, nil
}

func (s *repository) All(_ context.Context) ([]entity.Authorization, error) {
	out := make([]entity.Authorization, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.auths[id])
	}
	return out, nil
}
