package ledger_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// replay runs the canonical stream under a given fee-reversal policy and
// returns the engine. The policy is passed through the config rather than the
// environment, so tests never mutate process state and can run in parallel.
func replay(t *testing.T, policy config.FeeReversalPolicy) ledger.Service {
	t.Helper()
	cfg := config.Default()
	cfg.FeeReversal = policy
	svc := ledger.New(cfg, ledger.CanonicalAccounts()...)
	assert.NoError(t, svc.Replay(ledger.CanonicalStream()))
	return svc
}

// row returns one account's report row for one day.
func row(t *testing.T, svc ledger.Service, accountID string, day entity.Day) entity.DayReport {
	t.Helper()
	for _, r := range svc.Report() {
		if r.AccountID == accountID && r.Day == day {
			return r
		}
	}
	t.Fatalf("no report row for %s day %d", accountID, day)
	return entity.DayReport{}
}

// aed and bhd build expected amounts in the test's own words, so an assertion
// reads as the figure it is checking rather than as a constructor call.
func aed(s string) entity.Money { return entity.MustParseMoney(s, entity.AED) }
func bhd(s string) entity.Money { return entity.MustParseMoney(s, entity.BHD) }

// assertMoney compares by value and prints both sides in the currency's own
// scale on failure. Comparing the rendered strings instead would pass a BHD
// amount off against an AED one whenever the digits happened to match.
func assertMoney(t *testing.T, want, got entity.Money, format string, args ...any) {
	t.Helper()
	context := ""
	if format != "" {
		context = " -- " + fmt.Sprintf(format, args...)
	}
	assert.True(t, want.Equal(got),
		"want %s, got %s%s", want.Display(), got.Display(), context)
}

// feesOn returns the overdraft fees value-dated to a day, net of reversals.
func feesOn(svc ledger.Service, accountID string, day entity.Day) (assessed, reversed int) {
	for _, e := range svc.Entries() {
		if e.AccountID != accountID || e.ValueDate != day {
			continue
		}
		switch {
		case e.IsFee():
			assessed++
		case e.IsFeeReversal():
			reversed++
		}
	}
	return assessed, reversed
}

// countFees totals the overdraft fees on an account across the window.
func countFees(svc ledger.Service, accountID string) (assessed, reversed int) {
	for _, e := range svc.Entries() {
		if e.AccountID != accountID {
			continue
		}
		switch {
		case e.IsFee():
			assessed++
		case e.IsFeeReversal():
			reversed++
		}
	}
	return assessed, reversed
}
