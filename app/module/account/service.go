// Package account owns the accounts the ledger operates on: their currency and
// their opening balance. It is a module of its own because every other module
// needs to resolve an account ID and none of them should own that lookup.
package account

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Sentinel errors returned by the repository. The service translates them into
// coded errors; the repository stays free of the error taxonomy so a different
// store can reuse it unchanged.
var (
	ErrNotFound      = apperror.New(apperror.ErrUnknownAccount, "account not found")
	ErrAlreadyExists = apperror.New(apperror.ErrInvalidParameter, "account already exists")
)

type Service interface {
	Create(context.Context, entity.Account) (entity.Account, error)
	Read(context.Context, entity.AccountFilter) ([]entity.Account, error)
	Update(context.Context, entity.Account) (entity.Account, error)
	Delete(context.Context, string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, payload entity.Account) (entity.Account, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.Account{}, apperror.New(apperror.ErrUnexpected, "create account error", err)
	}
	return res, nil
}

func (s *service) Read(ctx context.Context, filter entity.AccountFilter) ([]entity.Account, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read account error", err)
	}
	return res, nil
}

func (s *service) Update(ctx context.Context, payload entity.Account) (entity.Account, error) {
	res, err := s.repo.Update(ctx, payload)
	if err != nil {
		return entity.Account{}, apperror.New(apperror.ErrUnexpected, "update account error", err)
	}
	return res, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.New(apperror.ErrUnexpected, "delete account error", err)
	}
	return nil
}
