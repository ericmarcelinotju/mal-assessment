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
)

// These are the tests the repository seam exists for.
//
// The behavioural tests in app/module/replay/test run every module against real
// in-memory repositories, because the arithmetic is the point there. Here the
// repository is mocked, so the service's own error handling can be exercised --
// the paths a working store never takes, which would otherwise be dead code
// that nobody has ever run.

func account() entity.Account {
	return entity.NewAccount("ACC-001", entity.AED, "0.00")
}

func creditEvent() entity.Event {
	return entity.Event{
		ID: "E1", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1,
		AccountID: "ACC-001", Amount: entity.MustParseMoney("100.00", entity.AED),
	}
}

func TestLedgerService_Post(t *testing.T) {
	t.Run("when the repository succeeds then the entry is returned", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		expected := entity.LedgerEntry{Seq: 1, EventID: "E1", AccountID: "ACC-001"}
		repo.EXPECT().
			AppendEntry(mock.Anything, mock.Anything).
			Return(expected, nil)

		res, err := svc.Post(ctx, account(), creditEvent(),
			entity.MustParseMoney("100.00", entity.AED), entity.OriginInstruction)

		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, expected.Seq, res[0].Seq)
	})

	t.Run("when the repository fails then the error is wrapped, not swallowed", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		repo.EXPECT().
			AppendEntry(mock.Anything, mock.Anything).
			Return(entity.LedgerEntry{}, gofakeit.ErrorDatabase())

		res, err := svc.Post(ctx, account(), creditEvent(),
			entity.MustParseMoney("100.00", entity.AED), entity.OriginInstruction)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "append ledger entry error",
			"the service names the operation that failed rather than passing the raw cause up")
	})

	t.Run("when an instalment credit is posted then one entry per part is appended", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		ev := creditEvent()
		ev.Amount = entity.MustParseMoney("10.000", entity.BHD)
		ev.Instalments = 3

		repo.EXPECT().
			AppendEntry(mock.Anything, mock.Anything).
			Return(entity.LedgerEntry{}, nil).
			Times(3)

		res, err := svc.Post(ctx, entity.NewAccount("B", entity.BHD, "0.000"), ev,
			entity.MustParseMoney("10.000", entity.BHD), entity.OriginInstruction)

		assert.NoError(t, err)
		assert.Len(t, res, 3)
	})
}

func TestLedgerService_Reads(t *testing.T) {
	t.Run("when reading entries fails then the error is wrapped", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		repo.EXPECT().
			Entries(mock.Anything).
			Return(nil, gofakeit.ErrorDatabase())

		res, err := svc.Entries(ctx)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "read ledger entries error")
	})

	t.Run("when the balance cannot read the journal then it does not report zero", func(t *testing.T) {
		// Returning a zero balance on a failed read would be the dangerous
		// failure mode: the fee sweep would see a non-negative balance and
		// silently decline to charge.
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		repo.EXPECT().
			Entries(mock.Anything).
			Return(nil, gofakeit.ErrorDatabase())

		_, err := svc.ClosingBalance(ctx, account(), 1)

		assert.Error(t, err)
	})
}

func TestLedgerService_Reject(t *testing.T) {
	t.Run("when an instruction is refused then the rejection is recorded", func(t *testing.T) {
		ctx := context.Background()

		repo := ledger_mock.NewMockRepository(t)
		svc := ledger.NewService(repo)

		repo.EXPECT().
			AppendError(mock.Anything, mock.Anything).
			Return(nil)

		err := svc.Reject(ctx, creditEvent(), "2001", "unknown account")

		assert.Error(t, err, "the caller still learns the instruction failed")
		assert.Contains(t, err.Error(), "unknown account")
	})
}
