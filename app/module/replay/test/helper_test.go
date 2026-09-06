package replay_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/account"
	"github.com/ericmarcelinotju/mal-assessment/app/module/authorization"
	"github.com/ericmarcelinotju/mal-assessment/app/module/fee"
	"github.com/ericmarcelinotju/mal-assessment/app/module/interest"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/app/module/rejection"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// stack is the wired set of modules under test. These are behavioural tests, so
// they run against real in-memory repositories throughout; the per-module tests
// use mocks to drive the failure paths instead.
type stack struct {
	replay     replay.Service
	ledger     ledger.Service
	interest   interest.Service
	auths      authorization.Service
	rejections rejection.Service
}

// newStack composes the modules the same way main does, so a wiring mistake in
// main is a wiring mistake here too.
func newStack(t *testing.T, policy config.FeeReversalPolicy, accounts ...entity.Account) stack {
	t.Helper()
	cfg := config.Default()
	cfg.FeeReversal = policy

	accountSvc := account.NewService(account.NewRepository())
	rejectionSvc := rejection.NewService(rejection.NewRepository())
	ledgerSvc := ledger.NewService(ledger.NewRepository(), rejectionSvc)
	authSvc := authorization.NewService(authorization.NewRepository(), ledgerSvc, rejectionSvc)
	feeSvc := fee.NewService(cfg, fee.NewRepository(), ledgerSvc)
	interestSvc := interest.NewService(cfg, interest.NewRepository(), ledgerSvc)

	for _, acc := range accounts {
		_, err := accountSvc.Create(context.Background(), acc)
		assert.NoError(t, err)
	}

	return stack{
		replay: replay.NewService(cfg, replay.NewRepository(),
			accountSvc, ledgerSvc, rejectionSvc, authSvc, feeSvc, interestSvc),
		rejections: rejectionSvc,
		ledger:     ledgerSvc,
		interest:   interestSvc,
		auths:      authSvc,
	}
}

// run replays the canonical stream under a given fee-reversal policy.
func run(t *testing.T, policy config.FeeReversalPolicy) stack {
	t.Helper()
	s := newStack(t, policy, replay.CanonicalAccounts()...)
	assert.NoError(t, s.replay.Run(context.Background(), replay.CanonicalStream()))
	return s
}

// runEvents replays an arbitrary stream against arbitrary accounts.
func runEvents(t *testing.T, accounts []entity.Account, events []entity.Event) stack {
	t.Helper()
	s := newStack(t, config.FeeReversalNone, accounts...)
	assert.NoError(t, s.replay.Run(context.Background(), events))
	return s
}

func report(t *testing.T, s stack) []entity.DayReport {
	t.Helper()
	rows, err := s.replay.Report(context.Background())
	assert.NoError(t, err)
	return rows
}

// row returns one account's report row for one day.
func row(t *testing.T, s stack, accountID string, day entity.Day) entity.DayReport {
	t.Helper()
	for _, r := range report(t, s) {
		if r.AccountID == accountID && r.Day == day {
			return r
		}
	}
	t.Fatalf("no report row for %s day %d", accountID, day)
	return entity.DayReport{}
}

func entries(t *testing.T, s stack) []entity.LedgerEntry {
	t.Helper()
	out, err := s.ledger.Read(context.Background(), entity.LedgerEntryFilter{})
	assert.NoError(t, err)
	return out
}

func accruals(t *testing.T, s stack) []entity.Accrual {
	t.Helper()
	out, err := s.interest.Read(context.Background(), entity.AccrualFilter{})
	assert.NoError(t, err)
	return out
}

func journalErrors(t *testing.T, s stack) []entity.LedgerError {
	t.Helper()
	out, err := s.rejections.Read(context.Background(), entity.RejectionFilter{})
	assert.NoError(t, err)
	return out
}

// auth looks up one authorization by ID.
func auth(t *testing.T, s stack, id string) (entity.Authorization, bool) {
	t.Helper()
	all, err := s.auths.Read(context.Background(), entity.AuthorizationFilter{})
	assert.NoError(t, err)
	for _, a := range all {
		if a.AuthID == id {
			return a, true
		}
	}
	return entity.Authorization{}, false
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
	note := ""
	if format != "" {
		note = " -- " + fmt.Sprintf(format, args...)
	}
	assert.True(t, want.Equal(got),
		"want %s, got %s%s", want.Display(), got.Display(), note)
}

// feesOn returns the overdraft fees value-dated to a day, net of reversals.
func feesOn(t *testing.T, s stack, accountID string, day entity.Day) (assessed, reversed int) {
	t.Helper()
	for _, e := range entries(t, s) {
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
func countFees(t *testing.T, s stack, accountID string) (assessed, reversed int) {
	t.Helper()
	for _, e := range entries(t, s) {
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

// mustAdd and mustSub keep the arithmetic in an assertion readable. A currency
// mismatch in a test is a broken test, so it fails immediately rather than
// being threaded through the assertion.
func mustAdd(t *testing.T, a, b entity.Money) entity.Money {
	t.Helper()
	res, err := a.Add(b)
	assert.NoError(t, err)
	return res
}

func mustSub(t *testing.T, a, b entity.Money) entity.Money {
	t.Helper()
	res, err := a.Sub(b)
	assert.NoError(t, err)
	return res
}
