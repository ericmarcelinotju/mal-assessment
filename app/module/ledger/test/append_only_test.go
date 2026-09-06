package ledger_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// TestAppendOnly_LogIsMonotonic checks the structural invariant: entries are
// only ever added, never rewritten or removed.
func TestAppendOnly_LogIsMonotonic(t *testing.T) {
	t.Run("when the replay runs then sequence numbers strictly increase", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		last := 0
		for _, e := range entries(t, svc) {
			assert.Greater(t, e.Seq, last, "the log is in append order with no reuse")
			last = e.Seq
		}
	})

	t.Run("when the log is read twice then it only grows", func(t *testing.T) {
		cfg := config.Default()
		svc := ledger.NewService(cfg, ledger.NewRepository(), ledger.CanonicalAccounts()...)

		var lengths []int
		for day := entity.Day(1); day <= 6; day++ {
			for _, ev := range ledger.CanonicalStream() {
				if ev.PostingDay == day {
					_, _ = svc.Post(context.Background(), ev)
				}
			}
			assert.NoError(t, svc.CloseDay(context.Background(), day))
			lengths = append(lengths, len(entries(t, svc)))
		}
		for i := 1; i < len(lengths); i++ {
			assert.GreaterOrEqual(t, lengths[i], lengths[i-1],
				"the log never shrinks between closes")
		}
	})
}

// TestAppendOnly_CallerCannotRewriteHistory guards the encapsulation. Entries()
// hands out a copy, so a caller that mutates what it receives cannot reach the
// engine's own records. Returning the internal slice would make "append-only" a
// convention rather than a property.
func TestAppendOnly_CallerCannotRewriteHistory(t *testing.T) {
	t.Run("when a caller mutates the returned slice then the ledger is unaffected", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		stolen := entries(t, svc)
		before := row(t, svc, ledger.ACC001, 6).ClosingBalance

		for i := range stolen {
			stolen[i].Amount = aed("999999.00")
		}

		assertMoney(t, before, row(t, svc, ledger.ACC001, 6).ClosingBalance,
			"a copy was handed out, not a handle on the log")
	})

	t.Run("when accruals are read then they are a copy too", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		stolen := accruals(t, svc)
		before := row(t, svc, ledger.ACC001, 2).InterestAccrued
		for i := range stolen {
			stolen[i].Amount = aed("42.00")
		}

		assertMoney(t, before, row(t, svc, ledger.ACC001, 2).InterestAccrued, "")
	})
}

// TestAppendOnly_CorrectionsAreContraEntries checks the behavioural half of the
// invariant: nothing is ever undone by removal.
func TestAppendOnly_CorrectionsAreContraEntries(t *testing.T) {
	t.Run("when E7 is reversed then E7 is still in the log", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		var found bool
		for _, e := range entries(t, svc) {
			if e.EventID == "E7" {
				found = true
				assertMoney(t, aed("-620.00"), e.Amount, "the original is unchanged")
			}
		}
		assert.True(t, found, "a reversed entry is corrected, not deleted")
	})

	t.Run("when fees are reversed then the fees are still in the log", func(t *testing.T) {
		svc := replay(t, config.FeeReversalOnCauseReversal)

		assessed, reversed := countFees(t, svc, ledger.ACC001)
		assert.Equal(t, 3, assessed, "the fee entries survive their own reversal")
		assert.Equal(t, 3, reversed)
	})

	t.Run("when the interest accrual is corrected then the original stands", func(t *testing.T) {
		svc := replay(t, config.FeeReversalNone)

		trail := row(t, svc, ledger.ACC001, 2).AccrualTrail
		assert.Len(t, trail, 3)
		assertMoney(t, aed("0.10"), trail[0].Amount,
			"the day 2 accrual still reads as it was written on day 2")
	})
}

// TestAppendOnly_ReplayIsDeterministic is the property that makes every figure
// in this repo checkable: no clock, no randomness, no map iteration order
// leaking into the result.
func TestAppendOnly_ReplayIsDeterministic(t *testing.T) {
	t.Run("when the same stream is replayed twice then the logs are identical", func(t *testing.T) {
		first := entries(t, replay(t, config.FeeReversalNone))
		second := entries(t, replay(t, config.FeeReversalNone))

		assert.Equal(t, len(first), len(second))
		for i := range first {
			assert.Equal(t, first[i].Seq, second[i].Seq)
			assert.Equal(t, first[i].EventID, second[i].EventID)
			assert.Equal(t, first[i].Amount.String(), second[i].Amount.String())
			assert.Equal(t, first[i].ValueDate, second[i].ValueDate)
			assert.Equal(t, first[i].PostingDay, second[i].PostingDay)
		}
	})
}

// TestAppendOnly_PostingOrderBeatsListedOrder covers the discrepancy in the
// brief: E9 is listed before E10 but posts a day later.
func TestAppendOnly_PostingOrderBeatsListedOrder(t *testing.T) {
	t.Run("when the stream is listed out of order then posting day decides", func(t *testing.T) {
		// A ledger cannot learn of a day 6 event before a day 5 one. The two
		// events touch different accounts so nothing here depends on it, but a
		// replay engine whose answer varies with input order is one that gives
		// different results for the same facts.
		svc := replay(t, config.FeeReversalNone)

		var e9, e10 int
		for _, e := range entries(t, svc) {
			switch e.EventID {
			case "E9":
				e9 = e.Seq
			case "E10":
				if e10 == 0 {
					e10 = e.Seq
				}
			}
		}
		assert.Less(t, e10, e9, "E10 posts on day 5, E9 on day 6, whatever the listing says")
	})

	t.Run("when the stream is shuffled then the result is unchanged", func(t *testing.T) {
		forward := replay(t, config.FeeReversalNone)

		reversed := make([]entity.Event, 0, 10)
		stream := ledger.CanonicalStream()
		for i := len(stream) - 1; i >= 0; i-- {
			reversed = append(reversed, stream[i])
		}
		shuffled := newService(config.FeeReversalNone, ledger.CanonicalAccounts()...)
		assert.NoError(t, shuffled.Replay(context.Background(), reversed))

		// Same-day ordering still comes from the listing, so a fully reversed
		// stream is not guaranteed to be identical -- but the closing position
		// must be, because no two same-day events on one account interact here.
		assertMoney(t,
			row(t, forward, ledger.ACC001, 6).ClosingBalance,
			row(t, shuffled, ledger.ACC001, 6).ClosingBalance,
			"the closing position must not depend on how the stream was listed")
	})
}
