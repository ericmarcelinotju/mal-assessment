package authorization

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository stores authorizations, approved and declined alike. A declined
// authorization is kept because the day report has to explain the decline;
// dropping it would leave an unexplained absence.
//
// This is the module with a genuine Update: an authorization's state moves from
// APPROVED to SETTLED when a settlement consumes it. That is a real mutation of
// a live record, not a rewrite of history, which is why it belongs here and not
// on the journal.
type Repository interface {
	Create(context.Context, entity.Authorization) (entity.Authorization, error)
	Read(context.Context, entity.AuthorizationFilter) ([]entity.Authorization, error)
	Update(context.Context, entity.Authorization) (entity.Authorization, error)
	Delete(context.Context, string) error
}

type repository struct {
	auths map[string]entity.Authorization
	order []string
}

func NewRepository() Repository {
	return &repository{auths: make(map[string]entity.Authorization)}
}

func (s *repository) Create(_ context.Context, a entity.Authorization) (entity.Authorization, error) {
	if _, exists := s.auths[a.AuthID]; exists {
		return entity.Authorization{}, ErrAlreadyExists
	}
	s.auths[a.AuthID] = a
	s.order = append(s.order, a.AuthID)
	return a, nil
}

func (s *repository) Read(
	_ context.Context, filter entity.AuthorizationFilter,
) ([]entity.Authorization, error) {
	out := make([]entity.Authorization, 0, len(s.order))
	for _, id := range s.order {
		a := s.auths[id]
		if filter.AuthID != "" && filter.AuthID != a.AuthID {
			continue
		}
		if filter.AccountID != "" && filter.AccountID != a.AccountID {
			continue
		}
		if filter.PostingDay != 0 && filter.PostingDay != a.PostingDay {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *repository) Update(_ context.Context, a entity.Authorization) (entity.Authorization, error) {
	if _, exists := s.auths[a.AuthID]; !exists {
		return entity.Authorization{}, ErrNotFound
	}
	s.auths[a.AuthID] = a
	return a, nil
}

func (s *repository) Delete(_ context.Context, id string) error {
	if _, exists := s.auths[id]; !exists {
		return ErrNotFound
	}
	delete(s.auths, id)
	for i, existing := range s.order {
		if existing == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}
