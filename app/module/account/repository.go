package account

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Repository stores the accounts the ledger operates on.
type Repository interface {
	Save(context.Context, entity.Account) error
	Get(context.Context, string) (entity.Account, error)
	All(context.Context) ([]entity.Account, error)
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

func (s *repository) Save(_ context.Context, acc entity.Account) error {
	if _, exists := s.accounts[acc.ID]; !exists {
		s.order = append(s.order, acc.ID)
	}
	s.accounts[acc.ID] = acc
	return nil
}

func (s *repository) Get(_ context.Context, id string) (entity.Account, error) {
	acc, ok := s.accounts[id]
	if !ok {
		return entity.Account{}, ErrNotFound
	}
	return acc, nil
}

func (s *repository) All(_ context.Context) ([]entity.Account, error) {
	out := make([]entity.Account, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.accounts[id])
	}
	return out, nil
}
