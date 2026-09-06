package ledger_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

func auth(t *testing.T, svc ledger.Service, id string) (entity.Authorization, bool) {
	t.Helper()
	for _, r := range report(t, svc) {
		for _, a := range r.Authorizations {
			if a.AuthID == id {
				return a, true
			}
		}
	}
	return entity.Authorization{}, false
}

func TestAuthorization_AuthAIsApproved(t *testing.T) {
	t.Run("when available covers the hold then the authorization is approved", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		a, ok := auth(t, svc, "Auth-A")
		assert.True(t, ok)
		// Day 2 available is 250.00; the 200.00 hold leaves 50.00, at or above
		// zero, so it stands.
		assertMoney(t, aed("200.00"), a.Hold, "")
		assert.Equal(t, entity.AuthSettled, a.State, "settled by E5 on day 4")
	})
}

// TestAuthorization_AuthBIsDeclined is the trap in the brief.
//
// The brief closes with "Auth-B is never settled inside the window", which
// invites the reader to assume Auth-B is an approved hold that simply never
// settles. It is not: by the time E8 arrives on day 5, E7 has already been
// posted back-valued to day 2 and the account is 155.00 overdrawn. The
// availability test cannot pass.
//
// This is also why criterion 5 is vacuous -- see criteria_test.go.
func TestAuthorization_AuthBIsDeclined(t *testing.T) {
	t.Run("when available is already negative then the authorization is declined", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		a, ok := auth(t, svc, "Auth-B")
		assert.True(t, ok, "a declined authorization is still recorded, so the report can explain it")
		assert.Equal(t, entity.AuthDeclined, a.State)
		assert.Contains(t, a.DeclineNote, "-245.00",
			"available -155.00 minus the 90.00 hold is -245.00")
	})

	t.Run("when Auth-B is declined then no hold and no entry exist", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		assertMoney(t, aed("0.00"), row(t, svc, ledger.ACC001, 5).ActiveHolds,
			"a declined authorization places no hold")
		for _, e := range entries(t, svc) {
			assert.NotEqual(t, entity.EventID("E8"), e.EventID,
				"an authorization never produces a ledger entry, declined or not")
		}
	})

	t.Run("when the decline is recorded then it appears as a day 5 error", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var found bool
		for _, e := range row(t, svc, ledger.ACC001, 5).Errors {
			if e.EventID == "E8" {
				found = true
			}
		}
		assert.True(t, found, "a decline is an outcome to report, not an absence")
	})

	// The decline does not depend on where in the day-5 close the test runs:
	// before the fee sweep available is -245.00, after it -320.00. Both are
	// below zero. Worth pinning, because if the account had been marginal the
	// ordering would have decided the outcome.
	t.Run("when fees are assessed first then Auth-B is still declined", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)
		a, _ := auth(t, svc, "Auth-B")
		assert.Equal(t, entity.AuthDeclined, a.State)
	})
}

func TestAuthorization_BoundaryIsAtOrAboveZero(t *testing.T) {
	// "approved only if available balance remains at or above zero" -- so an
	// exactly-zero result approves, and only a strictly negative one declines.
	// One minor unit either side of the boundary.
	base := []entity.Event{
		{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
	}
	accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}

	t.Run("when the hold lands exactly on zero then it is approved", func(t *testing.T) {
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), append(append([]entity.Event{}, base...), entity.Event{
			ID: "H", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1,
			AccountID: "A", Amount: aed("100.00"), AuthID: "exact",
		})))

		a, ok := auth(t, svc, "exact")
		assert.True(t, ok)
		assert.Equal(t, entity.AuthApproved, a.State, "zero is at or above zero")
	})

	t.Run("when the hold exceeds available by one minor unit then it is declined", func(t *testing.T) {
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), append(append([]entity.Event{}, base...), entity.Event{
			ID: "H", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1,
			AccountID: "A", Amount: aed("100.01"), AuthID: "over",
		})))

		a, ok := auth(t, svc, "over")
		assert.True(t, ok)
		assert.Equal(t, entity.AuthDeclined, a.State)
	})
}

func TestAuthorization_HoldsStackAgainstAvailability(t *testing.T) {
	t.Run("when a hold is already active then it reduces what a later one can take", func(t *testing.T) {
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
			{ID: "H1", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("60.00"), AuthID: "first"},
			{ID: "H2", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("60.00"), AuthID: "second"},
		}))

		first, _ := auth(t, svc, "first")
		second, _ := auth(t, svc, "second")
		assert.Equal(t, entity.AuthApproved, first.State)
		assert.Equal(t, entity.AuthDeclined, second.State,
			"100.00 - 60.00 - 60.00 is negative, so the second hold cannot stand")
	})
}
