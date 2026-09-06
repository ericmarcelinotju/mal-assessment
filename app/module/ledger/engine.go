package ledger

import (
	"sort"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Replay posts every event and closes every day of the window.
//
// Events are ordered by posting day, then by their position in the supplied
// stream. This matters because the brief lists E9 (posting day 6) before E10
// (posting day 5), so "replayed in this order" and "replayed in time order"
// disagree. Posting day wins: a ledger cannot learn of a Day 6 event before a
// Day 5 one. Within a single posting day the listed order is preserved, so the
// brief's sequencing is honoured wherever it is not self-contradictory. The two
// events involved touch different accounts, so on this stream the choice
// changes nothing -- but a replay engine that silently depends on input order
// is one that produces different answers for the same facts. See AMBIGUITIES.md.
//
// Rejected instructions do not abort the replay. A declined authorization and
// an orphan settlement are recorded outcomes, not failures of the engine, and
// the remaining events still have to be processed.
func (s *service) Replay(events []entity.Event) error {
	ordered := make([]entity.Event, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].PostingDay < ordered[j].PostingDay
	})

	next := 0
	for day := entity.Day(1); day <= entity.Day(s.cfg.WindowDays); day++ {
		for next < len(ordered) && ordered[next].PostingDay == day {
			//nolint:errcheck // rejections are recorded on the log and reported per day
			_, _ = s.Post(ordered[next])
			next++
		}
		if err := s.CloseDay(day); err != nil {
			return err
		}
	}
	return nil
}

// CloseDay runs the end-of-day cycle for every account.
//
// Order within the close is deliberate:
//
//  1. Assess overdraft fees. Fees are ledger entries, so they must exist before
//     interest is computed on the balance that includes them.
//  2. Reverse fees whose cause has gone away, if the policy allows it. This has
//     to follow assessment so a fee booked moments ago on a day that is still
//     genuinely negative is not immediately undone.
//  3. Accrue interest, restating earlier days whose balances have moved.
//  4. On the capitalisation day, book the single interest credit.
//
// The capitalisation credit is deliberately last, after both the fee sweep and
// the accrual. It is interest on the window, not part of the window's balances,
// so letting it feed back into the Day 6 fee test or into its own accrual would
// be circular. Both accounts close Day 6 positive, so this ordering is not
// exercised here -- but an account closing Day 6 slightly negative would make it
// decide whether interest can rescue a day from its overdraft fee. It cannot.
// See AMBIGUITIES.md.
func (s *service) CloseDay(day entity.Day) error {
	for _, id := range s.order {
		acc := s.accounts[id]

		if err := s.assessOverdraftFees(acc, day); err != nil {
			return err
		}
		if err := s.reverseFeesOnCauseReversal(acc, day); err != nil {
			return err
		}
		// The reversal sweep changes balances, so interest is accrued against
		// the post-sweep position.
		if err := s.accrueInterest(acc, day); err != nil {
			return err
		}
		if day == entity.Day(s.cfg.CapitalisationDay) {
			if err := s.capitalise(acc, day); err != nil {
				return err
			}
		}
	}
	return nil
}

// Report builds one row per day per account from the log. Everything here is
// derived on demand; no figure is cached at close time, so the report cannot
// drift from the entries that justify it.
func (s *service) Report() []entity.DayReport {
	var out []entity.DayReport

	for day := entity.Day(1); day <= entity.Day(s.cfg.WindowDays); day++ {
		for _, id := range s.order {
			acc := s.accounts[id]

			row := entity.DayReport{
				Day:              day,
				AccountID:        acc.ID,
				Currency:         acc.Currency,
				ClosingBalance:   s.closingBalance(acc, day),
				ActiveHolds:      s.activeHolds(acc, day),
				AvailableBalance: s.availableBalance(acc, day),
				InterestAccrued:  s.netAccrual(acc.ID, day),
			}

			for _, e := range s.log.entries {
				if e.AccountID != acc.ID || e.ValueDate != day {
					continue
				}
				switch {
				case e.Origin == entity.OriginOverdraftFee:
					row.FeesAssessed = append(row.FeesAssessed, e)
				case e.Origin == entity.OriginFeeReversal:
					row.FeesReversed = append(row.FeesReversed, e)
				case e.Origin == entity.OriginCapitalisation:
					cap := e
					row.Capitalisation = &cap
				}
				if e.Origin != entity.OriginCapitalisation {
					row.Entries = append(row.Entries, e)
				}
			}

			for _, a := range s.log.accruals {
				if a.AccountID == acc.ID && a.Day == day {
					row.AccrualTrail = append(row.AccrualTrail, a)
				}
			}
			for _, a := range s.authorizations() {
				if a.AccountID == acc.ID && a.PostingDay == day {
					row.Authorizations = append(row.Authorizations, a)
				}
			}
			for _, e := range s.log.errors {
				if e.AccountID == acc.ID && e.Day == day {
					row.Errors = append(row.Errors, e)
				}
			}

			out = append(out, row)
		}
	}
	return out
}
