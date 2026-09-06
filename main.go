// Command mal-assessment replays the six-day event stream from the brief
// against the in-memory ledger core and prints the per-day report.
//
//	go run main.go
//	LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go
//
// There is no server, no database and no UI. The modules are a pure function of
// the event stream, so this program is deterministic: same input, same output,
// every run.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ericmarcelinotju/mal-assessment/app/module/account"
	"github.com/ericmarcelinotju/mal-assessment/app/module/authorization"
	"github.com/ericmarcelinotju/mal-assessment/app/module/fee"
	"github.com/ericmarcelinotju/mal-assessment/app/module/interest"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/app/module/replay"
	"github.com/ericmarcelinotju/mal-assessment/app/presenter"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// main is a composition root and nothing else: build each module's repository,
// build its service over that repository, and wire the modules together in
// dependency order. No business logic lives here.
//
// The order below is the dependency graph read aloud. account and ledger have
// no dependencies; authorization, fee and interest each depend on ledger;
// replay depends on all of them and nothing depends on replay.
func main() {
	ctx := context.Background()
	cfg := config.Load()

	accountSvc := account.NewService(account.NewRepository())
	ledgerSvc := ledger.NewService(ledger.NewRepository())
	authSvc := authorization.NewService(authorization.NewRepository(), ledgerSvc)
	feeSvc := fee.NewService(cfg, fee.NewRepository(), ledgerSvc)
	interestSvc := interest.NewService(cfg, interest.NewRepository(), ledgerSvc)

	replaySvc := replay.NewService(
		cfg, replay.NewRepository(),
		accountSvc, ledgerSvc, authSvc, feeSvc, interestSvc,
	)

	for _, acc := range replay.CanonicalAccounts() {
		if err := accountSvc.Register(ctx, acc); err != nil {
			fail(err)
		}
	}

	if err := replaySvc.Run(ctx, replay.CanonicalStream()); err != nil {
		// A rejected instruction is not a failure and never reaches here -- it
		// is recorded on the journal and reported under its own day. This is
		// for the case where a module itself cannot proceed.
		fail(err)
	}

	rows, err := replaySvc.Report(ctx)
	if err != nil {
		fail(err)
	}
	entries, err := ledgerSvc.Entries(ctx)
	if err != nil {
		fail(err)
	}

	presenter.Render(os.Stdout, cfg, rows, entries)

	fmt.Fprintln(os.Stdout, "\nRun `go test ./...` for the acceptance-criteria suite "+
		"(one failure is expected; see README). REJECTED.md gives the criteria "+
		"that were refused and why.")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "replay failed:", err)
	os.Exit(1)
}
