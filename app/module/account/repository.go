package account

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository stores the accounts the ledger operates on.
type Repository interface {
	Create(context.Context, entity.Account) (entity.Account, error)
	Read(context.Context, entity.AccountFilter) ([]entity.Account, error)
	Update(context.Context, entity.Account) (entity.Account, error)
	Delete(context.Context, string) error
}

// repository keeps the accounts in memory, preserving insertion order so the
// report renders them in a stable sequence rather than in map order.
type repository struct {
	accounts map[string]entity.Account
	order    []string
}

func NewRepository() Repository {
	return &repository{accounts: make(map[string]entity.Account)}
}

func (s *repository) Create(_ context.Context, acc entity.Account) (entity.Account, error) {
	if _, exists := s.accounts[acc.ID]; exists {
		return entity.Account{}, ErrAlreadyExists
	}
	s.accounts[acc.ID] = acc
	s.order = append(s.order, acc.ID)
	return acc, nil
}

func (s *repository) Read(_ context.Context, filter entity.AccountFilter) ([]entity.Account, error) {
	out := make([]entity.Account, 0, len(s.order))
	for _, id := range s.order {
		if filter.ID != "" && filter.ID != id {
			continue
		}
		out = append(out, s.accounts[id])
	}
	return out, nil
}

func (s *repository) Update(_ context.Context, acc entity.Account) (entity.Account, error) {
	if _, exists := s.accounts[acc.ID]; !exists {
		return entity.Account{}, ErrNotFound
	}
	s.accounts[acc.ID] = acc
	return acc, nil
}

func (s *repository) Delete(_ context.Context, id string) error {
	if _, exists := s.accounts[id]; !exists {
		return ErrNotFound
	}
	delete(s.accounts, id)
	for i, existing := range s.order {
		if existing == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}
