// Command mal-assessment replays the six-day event stream from the brief
// against the in-memory ledger core and prints the per-day report.
//
//	go run main.go
//	LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal go run main.go
//
// There is no server, no database and no UI. The engine is a pure function of
// the event stream, so this program is deterministic: same input, same output,
// every run.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/app/presenter"
	"github.com/ericmarcelinotju/mal-assessment/config"
	"github.com/ericmarcelinotju/mal-assessment/logger"
)

func main() {
	slog.SetDefault(slog.New(logger.NewContextHandler(logger.ENV_LOCAL)))

	cfg := config.Load()
	svc := ledger.New(cfg, ledger.CanonicalAccounts()...)

	if err := svc.Replay(ledger.CanonicalStream()); err != nil {
		// A rejected instruction is not an engine failure and does not reach
		// here; it is recorded and reported per day. This is for the case where
		// the engine itself cannot proceed.
		slog.Error("replay failed", slog.Any("error", err))
		os.Exit(1)
	}

	presenter.Render(os.Stdout, cfg, svc.Report(), svc.Entries())

	fmt.Fprintf(os.Stdout, "\n%s\n", "Run `make test` for the acceptance-criteria suite. "+
		"See REJECTED.md for the criteria that were refused and why.")
}
