package ledger_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// newService builds a service over a real in-memory repository. Tests that want
// to drive the repository's failure paths use the generated mock instead -- see
// service_test.go.
func newService(policy config.FeeReversalPolicy, accounts ...entity.Account) ledger.Service {
	cfg := config.Default()
	cfg.FeeReversal = policy
	return ledger.NewService(cfg, ledger.NewRepository(), accounts...)
}

// replay runs the canonical stream under a given fee-reversal policy. The
// policy is passed through the config rather than the environment, so tests
// never mutate process state.
func replay(t *testing.T, policy config.FeeReversalPolicy) ledger.Service {
	t.Helper()
	svc := newService(policy, ledger.CanonicalAccounts()...)
	assert.NoError(t, svc.Replay(context.Background(), ledger.CanonicalStream()))
	return svc
}

// replayEvents runs an arbitrary stream against arbitrary accounts.
func replayEvents(t *testing.T, accounts []entity.Account, events []entity.Event) ledger.Service {
	t.Helper()
	svc := newService(config.FeeReversalNone, accounts...)
	assert.NoError(t, svc.Replay(context.Background(), events))
	return svc
}

// report returns the whole report, failing the test if the service cannot
// produce one.
func report(t *testing.T, svc ledger.Service) []entity.DayReport {
	t.Helper()
	rows, err := svc.Report(context.Background())
	assert.NoError(t, err)
	return rows
}

// row returns one account's report row for one day.
func row(t *testing.T, svc ledger.Service, accountID string, day entity.Day) entity.DayReport {
	t.Helper()
	for _, r := range report(t, svc) {
		if r.AccountID == accountID && r.Day == day {
			return r
		}
	}
	t.Fatalf("no report row for %s day %d", accountID, day)
	return entity.DayReport{}
}

// entries returns the append-only log.
func entries(t *testing.T, svc ledger.Service) []entity.LedgerEntry {
	t.Helper()
	out, err := svc.Entries(context.Background())
	assert.NoError(t, err)
	return out
}

func accruals(t *testing.T, svc ledger.Service) []entity.Accrual {
	t.Helper()
	out, err := svc.Accruals(context.Background())
	assert.NoError(t, err)
	return out
}

func ledgerErrors(t *testing.T, svc ledger.Service) []entity.LedgerError {
	t.Helper()
	out, err := svc.Errors(context.Background())
	assert.NoError(t, err)
	return out
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
func feesOn(t *testing.T, svc ledger.Service, accountID string, day entity.Day) (assessed, reversed int) {
	t.Helper()
	for _, e := range entries(t, svc) {
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
func countFees(t *testing.T, svc ledger.Service, accountID string) (assessed, reversed int) {
	t.Helper()
	for _, e := range entries(t, svc) {
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
