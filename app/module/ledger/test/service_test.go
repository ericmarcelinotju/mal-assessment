package ledger_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	ledger_mock "github.com/ericmarcelinotju/mal-assessment/app/module/ledger/mock"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// These are the tests the repository seam exists for.
//
// Every other file in this package drives the service through a real in-memory
// repository, because the arithmetic is the point there. Here the repository is
// mocked, so the service's own error handling can be exercised -- the paths a
// working store never takes and which would otherwise be dead code that nobody
// has ever run.

func mockedService(t *testing.T, repo *ledger_mock.MockRepository) ledger.Service {
	t.Helper()
	return ledger.NewService(config.Default(), repo, ledger.CanonicalAccounts()...)
}

func TestLedgerService_Post(t *testing.T) {
	t.Run("when the repository succeeds then the entry is returned", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		expected := entity.LedgerEntry{Seq: 1, EventID: "E1", AccountID: ledger.ACC001}
		repo.EXPECT().
			AppendEntry(mock.Anything, mock.Anything).
			Return(expected, nil)

		res, err := svc.Post(ctx, entity.Event{
			ID: "E1", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1,
			AccountID: ledger.ACC001, Amount: aed("100.00"),
		})

		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, expected.Seq, res[0].Seq)
	})

	t.Run("when the repository fails then the error is wrapped, not swallowed", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		repo.EXPECT().
			AppendEntry(mock.Anything, mock.Anything).
			Return(entity.LedgerEntry{}, gofakeit.ErrorDatabase())

		res, err := svc.Post(ctx, entity.Event{
			ID: "E1", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1,
			AccountID: ledger.ACC001, Amount: aed("100.00"),
		})

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "append ledger entry error",
			"the service names the operation that failed rather than passing the raw cause up")
	})

	t.Run("when the account is unknown then nothing is appended", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		// Only the rejection is recorded. AppendEntry is never expected, and
		// mockery fails the test at cleanup if it is called anyway.
		repo.EXPECT().
			AppendError(mock.Anything, mock.Anything).
			Return(nil)

		res, err := svc.Post(ctx, entity.Event{
			ID: "X", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1,
			AccountID: "NOPE", Amount: aed("100.00"),
		})

		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestLedgerService_Reads(t *testing.T) {
	t.Run("when reading entries fails then the error is wrapped", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		repo.EXPECT().
			Entries(mock.Anything).
			Return(nil, gofakeit.ErrorDatabase())

		res, err := svc.Entries(ctx)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "read ledger entries error")
	})

	t.Run("when reading accruals fails then the error is wrapped", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		repo.EXPECT().
			Accruals(mock.Anything).
			Return(nil, gofakeit.ErrorDatabase())

		res, err := svc.Accruals(ctx)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "read accruals error")
	})
}

func TestLedgerService_CloseDay(t *testing.T) {
	t.Run("when the day close cannot read the log then it stops", func(t *testing.T) {
		// A failing read during the overdraft sweep must abort the close rather
		// than carry on against a balance it could not compute. Getting this
		// wrong would assess fees off a zero balance.
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := mockedService(t, repo)

		repo.EXPECT().
			Entries(mock.Anything).
			Return(nil, gofakeit.ErrorDatabase())

		err := svc.CloseDay(ctx, 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read ledger entries error")
	})
}
