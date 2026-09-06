// Package presenter renders the ledger report as text. The brief forbids a UI,
// so this is the whole of the output layer: it formats, it does not compute.
// Every figure it prints comes from the engine already decided.
package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

const rule = "────────────────────────────────────────────────────────────────────────────"

// Render writes the per-day report: closing ledger balance, fee assessments,
// authorization states and errors, for each account on each day.
func Render(w io.Writer, cfg config.Config, rows []entity.DayReport, entries []entity.LedgerEntry) {
	fmt.Fprintln(w, rule)
	fmt.Fprintln(w, "IN-MEMORY ACCOUNT LEDGER — six-day replay")
	fmt.Fprintf(w, "fee reversal policy : %s", cfg.FeeReversal)
	switch cfg.FeeReversal {
	case config.FeeReversalNone:
		fmt.Fprint(w, "   (literal reading of the brief; criterion 6 is FALSE)\n")
	case config.FeeReversalOnCauseReversal:
		fmt.Fprint(w, "   (fees de-assessed when their cause is reversed; criterion 6 is TRUE)\n")
	}
	fmt.Fprintf(w, "interest rate       : %d/%d per day on positive closing balances\n",
		cfg.InterestRateNum, cfg.InterestRateDen)
	fmt.Fprintln(w, rule)

	var currentDay entity.Day
	for _, r := range rows {
		if r.Day != currentDay {
			currentDay = r.Day
			fmt.Fprintf(w, "\n\nDAY %d\n%s\n", r.Day, strings.Repeat("=", 76))
		}
		renderRow(w, r)
	}

	renderLog(w, entries)
}

func renderRow(w io.Writer, r entity.DayReport) {
	fmt.Fprintf(w, "\n  %s  (%s)\n", r.AccountID, r.Currency)

	fmt.Fprintf(w, "    closing ledger balance   %14s\n", r.ClosingBalance)
	fmt.Fprintf(w, "    active holds             %14s\n", r.ActiveHolds)
	fmt.Fprintf(w, "    available balance        %14s\n", r.AvailableBalance)

	// Fee assessments.
	switch {
	case len(r.FeesAssessed) == 0 && len(r.FeesReversed) == 0:
		fmt.Fprintf(w, "    overdraft fee            %14s\n", "none")
	default:
		for _, f := range r.FeesAssessed {
			fmt.Fprintf(w, "    overdraft fee            %14s   assessed on day %d\n",
				f.Amount, f.PostingDay)
		}
		for _, f := range r.FeesReversed {
			fmt.Fprintf(w, "    fee reversal             %14s   booked on day %d\n",
				f.Amount, f.PostingDay)
		}
	}

	// Interest, with the append-only restatement trail when there is one.
	fmt.Fprintf(w, "    interest accrued (net)   %14s\n", r.InterestAccrued)
	if len(r.AccrualTrail) > 1 {
		for _, a := range r.AccrualTrail {
			label := "accrued"
			if a.IsAdjustment {
				label = "adjusted"
			}
			fmt.Fprintf(w, "        %-9s %12s   booked day %d, on balance %s\n",
				label, a.Amount, a.BookedOnDay, a.Basis)
		}
	}

	if r.Capitalisation != nil {
		fmt.Fprintf(w, "    interest capitalised     %14s   single credit, sum of daily accruals\n",
			r.Capitalisation.Amount)
	}

	// Entries value-dated to this day.
	if len(r.Entries) > 0 {
		fmt.Fprintln(w, "    entries")
		for _, e := range r.Entries {
			backdated := ""
			if e.PostingDay != e.ValueDate {
				backdated = fmt.Sprintf("  [posted day %d, back-valued]", e.PostingDay)
			}
			line := fmt.Sprintf("%-7s %14s  %-26s%s", e.EventID, e.Amount, e.Memo, backdated)
			fmt.Fprintf(w, "        %s\n", strings.TrimRight(line, " "))
		}
	}

	// Authorization states.
	if len(r.Authorizations) > 0 {
		fmt.Fprintln(w, "    authorizations")
		for _, a := range r.Authorizations {
			fmt.Fprintf(w, "        %-8s %-9s hold %s", a.AuthID, a.State, a.Hold)
			switch a.State {
			case entity.AuthDeclined:
				fmt.Fprintf(w, "   %s", a.DeclineNote)
			case entity.AuthSettled:
				fmt.Fprintf(w, "   settled %s on day %d, hold released in full",
					a.SettledAmount, a.SettledDay)
			}
			fmt.Fprintln(w)
		}
	}

	// Errors.
	if len(r.Errors) == 0 {
		fmt.Fprintf(w, "    errors                   %14s\n", "none")
	} else {
		fmt.Fprintln(w, "    errors")
		for _, e := range r.Errors {
			fmt.Fprintf(w, "        %-4s [%s] %s\n", e.EventID, e.Code, e.Message)
		}
	}
}

// renderLog prints the append-only log in append order. Both clocks are shown,
// so a back-value posting is visible as such rather than having to be inferred.
func renderLog(w io.Writer, entries []entity.LedgerEntry) {
	fmt.Fprintf(w, "\n\n%s\nAPPEND-ONLY LOG  (%d entries, in append order; nothing was mutated or removed)\n%s\n",
		rule, len(entries), rule)
	fmt.Fprintf(w, "  %-4s  %-8s  %-7s  %-6s  %-5s  %14s  %s\n",
		"seq", "account", "event", "posted", "value", "amount", "origin / memo")
	for _, e := range entries {
		flag := " "
		if e.PostingDay != e.ValueDate {
			flag = "*"
		}
		fmt.Fprintf(w, "  %-4d  %-8s  %-7s  day %d%s  day %d  %14s  %s — %s\n",
			e.Seq, e.AccountID, e.EventID, e.PostingDay, flag, e.ValueDate, e.Amount, e.Origin, e.Memo)
	}
	// Sequence numbers are shared with the accrual records, which is why they
	// are not contiguous here: seq is a total order over the whole log, not an
	// index into this table.
	fmt.Fprintln(w, "\n  * posting day differs from value date: a back-value posting.")
}
