// Package account owns the accounts the ledger operates on: their currency and
// their opening balance. It is a module of its own because every other module
// needs to resolve an account ID and none of them should own that lookup.
package account

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// ErrNotFound is returned by the repository for an unknown account ID. The
// service turns it into a coded error; the repository stays free of the error
// taxonomy so a different store can reuse it.
var ErrNotFound = apperror.New(apperror.ErrUnknownAccount, "account not found")

type Service interface {
	Register(context.Context, entity.Account) error
	Get(context.Context, string) (entity.Account, error)
	All(context.Context) ([]entity.Account, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, acc entity.Account) error {
	if err := s.repo.Save(ctx, acc); err != nil {
		return apperror.New(apperror.ErrUnexpected, "register account error", err)
	}
	return nil
}

func (s *service) Get(ctx context.Context, id string) (entity.Account, error) {
	acc, err := s.repo.Get(ctx, id)
	if err != nil {
		return entity.Account{}, apperror.New(apperror.ErrUnknownAccount, "unknown account "+id, err)
	}
	return acc, nil
}

func (s *service) All(ctx context.Context) ([]entity.Account, error) {
	accounts, err := s.repo.All(ctx)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read accounts error", err)
	}
	return accounts, nil
}
