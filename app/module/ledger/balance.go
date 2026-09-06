package ledger

import (
	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// entryFilter selects which entries participate in a balance. Every balance in
// this engine is "the sum of the entries that pass a filter", which keeps the
// several variants the rules demand from drifting apart.
type entryFilter func(entity.LedgerEntry) bool

// closingBalance is the closing ledger balance for a day: the opening balance
// plus every entry with value_date <= day.
//
// This is the definition the brief gives in parentheses, and it is the reason
// criterion 2 cannot be satisfied. A back-value entry does not land on one day;
// it is a member of the value_date <= d set for its own day and for every
// later day, so it depresses all of them at once.
func (s *service) closingBalance(acc entity.Account, day entity.Day) entity.Money {
	return s.balanceWhere(acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day
	})
}

// closingBalanceAsObserved is the same figure restricted to what the ledger
// knew at the end of a given processing day. Balances are bitemporal: the Day 2
// closing balance is +250.00 observed from Day 2 and -370.00 observed from
// Day 5, because E7 had not yet been posted on Day 2.
//
// Acceptance criterion 1 pins both coordinates -- "the Day 2 closing balance,
// evaluated at end of Day 5" -- which is what makes it answerable.
func (s *service) closingBalanceAsObserved(acc entity.Account, day, observedOn entity.Day) entity.Money {
	return s.balanceWhere(acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day && e.PostingDay <= observedOn
	})
}

// closingBalanceExcludingSeq is the closing balance with one entry left out.
// The fee reversal sweep needs it: the question "is this day still overdrawn?"
// has to be asked without the fee that is itself under consideration, or the
// fee would forever justify its own existence.
func (s *service) closingBalanceExcludingSeq(acc entity.Account, day entity.Day, seq int) entity.Money {
	return s.balanceWhere(acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day && e.Seq != seq
	})
}

func (s *service) balanceWhere(acc entity.Account, keep entryFilter) entity.Money {
	total := acc.Opening
	for _, e := range s.log.entries {
		if keep(e) {
			total = total.Add(e.Amount)
		}
	}
	return total
}

// activeHolds sums the holds outstanding for an account as at a day. A hold
// counts from its value date until the day it is settled; a declined
// authorization never counts.
func (s *service) activeHolds(acc entity.Account, day entity.Day) entity.Money {
	total := entity.Zero(acc.Currency)
	for _, a := range s.auths {
		if a.AccountID != acc.ID || a.ValueDate > day {
			continue
		}
		switch a.State {
		case entity.AuthApproved:
			total = total.Add(a.Hold)
		case entity.AuthSettled:
			// The hold stood until the settlement landed, so it still reduces
			// availability on the days before that.
			if day < a.SettledDay {
				total = total.Add(a.Hold)
			}
		}
	}
	return total
}

// availableBalance is the figure the authorization test uses: ledger balance
// minus active holds. Holds never touch the ledger balance itself -- that is
// the whole point of a hold, and the substance of criterion 5.
func (s *service) availableBalance(acc entity.Account, day entity.Day) entity.Money {
	return s.closingBalance(acc, day).Sub(s.activeHolds(acc, day))
}
