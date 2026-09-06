package ledger_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// TestSettlement_AuthAIsAccepted covers acceptance criterion 3.
func TestSettlement_AuthAIsAccepted(t *testing.T) {
	t.Run("when Auth-A settles then a 185.00 debit is booked on day 4", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var found bool
		for _, e := range entries(t, svc) {
			if e.EventID == "E5" {
				found = true
				assertMoney(t, aed("-185.00"), e.Amount, "")
				assert.Equal(t, entity.Day(4), e.ValueDate)
				assert.Equal(t, entity.OriginSettlement, e.Origin)
			}
		}
		assert.True(t, found, "the settlement must post")
	})

	t.Run("when the settlement is under the hold then the surplus is released", func(t *testing.T) {
		// A 200.00 hold settling 185.00 leaves 15.00 unused. The authorization
		// is closed, so there is nothing left for the residue to secure and it
		// is released rather than retained. See AMBIGUITIES.md on partial
		// release.
		svc := replay(t, config.FeeReversalNone)

		a, ok := auth(t, svc, "Auth-A")
		assert.True(t, ok)
		assert.Equal(t, entity.AuthSettled, a.State)
		assertMoney(t, aed("185.00"), a.SettledAmount, "")
		assertMoney(t, aed("0.00"), row(t, svc, ledger.ACC001, 4).ActiveHolds,
			"no residue of the 200.00 hold survives the settlement")
	})

	t.Run("when the account could not fund it then a settlement still posts", func(t *testing.T) {
		// Settlements are exempt from the availability test: the funds were
		// reserved at authorization, and a promise already made to a merchant is
		// not something the ledger may renege on at settlement time.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
			{ID: "H", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00"), AuthID: "X"},
			{ID: "D", Type: entity.EventDebit, PostingDay: 2, ValueDate: 2, AccountID: "A", Amount: aed("100.00")},
			{ID: "S", Type: entity.EventSettlement, PostingDay: 3, ValueDate: 3, AccountID: "A", Amount: aed("100.00"), AuthID: "X"},
		}))

		var posted bool
		for _, e := range entries(t, svc) {
			if e.EventID == "S" {
				posted = true
			}
		}
		assert.True(t, posted, "an authorized settlement posts even when it overdraws")
	})
}

// TestSettlement_OrphanIsRejected covers acceptance criterion 4.
func TestSettlement_OrphanIsRejected(t *testing.T) {
	t.Run("when the authorization is unknown then no funds move", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		for _, e := range entries(t, svc) {
			assert.NotEqual(t, entity.EventID("E6"), e.EventID,
				"E6 references Auth-Z, which never existed; it must produce no entry")
		}

		// Day 4 closes at 415.00. Had E6 been force-posted it would close at
		// 235.00 -- the 180.00 difference is the whole of this decision.
		assertMoney(t, aed("415.00"), row(t, svc, ledger.ACC001, 4).ClosingBalance, "")
	})

	t.Run("when a settlement is rejected then the rejection is recorded", func(t *testing.T) {
		// Rejection is an outcome to report, not silence. An operator needs to
		// know a settlement arrived and why nothing happened.
		svc := replay(t, config.FeeReversalNone)

		var found bool
		for _, e := range row(t, svc, ledger.ACC001, 4).Errors {
			if e.EventID == "E6" {
				found = true
				assert.Contains(t, e.Message, "no preceding authorization")
			}
		}
		assert.True(t, found)
	})

	t.Run("when the authorization was declined then settling it is refused", func(t *testing.T) {
		// A declined authorization is recorded but never approved, so a
		// settlement against it has no reserved funds behind it and is refused
		// on the same grounds as an orphan.
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "H", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("50.00"), AuthID: "X"},
			{ID: "S", Type: entity.EventSettlement, PostingDay: 2, ValueDate: 2, AccountID: "A", Amount: aed("50.00"), AuthID: "X"},
		}))

		for _, e := range entries(t, svc) {
			assert.NotEqual(t, entity.EventID("S"), e.EventID)
		}
	})

	t.Run("when an authorization is settled twice then the second is refused", func(t *testing.T) {
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("100.00")},
			{ID: "H", Type: entity.EventAuthorization, PostingDay: 1, ValueDate: 1, AccountID: "A", Amount: aed("50.00"), AuthID: "X"},
			{ID: "S1", Type: entity.EventSettlement, PostingDay: 2, ValueDate: 2, AccountID: "A", Amount: aed("50.00"), AuthID: "X"},
			{ID: "S2", Type: entity.EventSettlement, PostingDay: 3, ValueDate: 3, AccountID: "A", Amount: aed("50.00"), AuthID: "X"},
		}))

		var s1, s2 bool
		for _, e := range entries(t, svc) {
			switch e.EventID {
			case "S1":
				s1 = true
			case "S2":
				s2 = true
			}
		}
		assert.True(t, s1, "the first settlement consumes the hold")
		assert.False(t, s2, "the hold is spent; a second settlement has nothing behind it")
	})
}

func TestSettlement_ReversalIsAContraEntry(t *testing.T) {
	t.Run("when E9 reverses E7 then both entries stand", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var e7, e9 entity.LedgerEntry
		for _, e := range entries(t, svc) {
			switch e.EventID {
			case "E7":
				e7 = e
			case "E9":
				e9 = e
			}
		}

		assertMoney(t, aed("-620.00"), e7.Amount, "the reversed entry is never touched")
		assertMoney(t, aed("620.00"), e9.Amount, "the reversal is a new, opposite entry")
		assert.Equal(t, e7.Seq, e9.ReversesSeq, "the contra entry names what it undoes")
		assert.Greater(t, e9.Seq, e7.Seq, "the correction is appended after the error")
	})

	t.Run("when a reversal is booked then it takes the original value date", func(t *testing.T) {
		// E9 is posted on day 6 but value-dated day 2. Reversing at today's date
		// instead would make the current total look right while leaving every
		// historical day permanently wrong.
		svc := replay(t, config.FeeReversalNone)

		for _, e := range entries(t, svc) {
			if e.EventID == "E9" {
				assert.Equal(t, entity.Day(6), e.PostingDay)
				assert.Equal(t, entity.Day(2), e.ValueDate)
			}
		}
	})

	t.Run("when the reversal target is unknown then it is refused", func(t *testing.T) {
		accounts := []entity.Account{entity.NewAccount("A", entity.AED, "0.00")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "R", Type: entity.EventReversal, PostingDay: 1, ValueDate: 1, AccountID: "A", ReversesEventID: "nope"},
		}))

		assert.Empty(t, entries(t, svc))
		assert.NotEmpty(t, ledgerErrors(t, svc))
	})

	t.Run("when an instalment credit is reversed then every part is undone", func(t *testing.T) {
		// E10 posts three entries under one event ID. A reversal has to undo all
		// of them, or the account keeps a fraction of a credit that was withdrawn.
		accounts := []entity.Account{entity.NewAccount("B", entity.BHD, "0.000")}
		svc := newService(config.FeeReversalNone, accounts...)
		assert.NoError(t, svc.Replay(context.Background(), []entity.Event{
			{ID: "C", Type: entity.EventCredit, PostingDay: 1, ValueDate: 1, AccountID: "B", Amount: bhd("10.000"), Instalments: 3},
			{ID: "R", Type: entity.EventReversal, PostingDay: 2, ValueDate: 1, AccountID: "B", ReversesEventID: "C"},
		}))

		assertMoney(t, bhd("0.000"), row(t, svc, "B", 6).ClosingBalance,
			"all three instalments must be reversed, leaving nothing behind")
	})
}
