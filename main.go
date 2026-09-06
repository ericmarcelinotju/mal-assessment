// Command mal-assessment replays the six-day event stream from the brief
// against the in-memory ledger core and prints the per-day report.
//
//	go run main.go
//	LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go
//
// There is no server, no database and no UI. The service is a pure function of
// the event stream, so this program is deterministic: same input, same output,
// every run.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// main is a composition root and nothing else, the same shape the boilerplate
// uses: build the repository, build the service over it, hand both to the route
// layer. No business logic lives here.
func main() {
	ctx := context.Background()
	cfg := config.Load()

	ledgerRepo := ledger.NewRepository()
	ledgerSvc := ledger.NewService(cfg, ledgerRepo, ledger.CanonicalAccounts()...)

	if err := ledger.NewRoutes(ctx, os.Stdout, ledgerSvc); err != nil {
		// A rejected instruction is not a failure of the service and never
		// reaches here -- it is recorded on the log and reported under its own
		// day. This is for the case where the service itself cannot proceed.
		fmt.Fprintln(os.Stderr, "replay failed:", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "\nRun `go test ./...` for the acceptance-criteria suite "+
		"(one failure is expected; see README). REJECTED.md gives the criteria "+
		"that were refused and why.")
}
