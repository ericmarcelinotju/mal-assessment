package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// entryFilter selects which entries participate in a balance. Every balance in
// this service is "the sum of the entries that pass a filter", which keeps the
// several variants the rules demand from drifting apart.
type entryFilter func(entity.LedgerEntry) bool

// closingBalance is the closing ledger balance for a day: the opening balance
// plus every entry with value_date <= day.
//
// This is the definition the brief gives in parentheses, and it is the reason
// criterion 2 cannot be satisfied. A back-value entry does not land on one day;
// it is a member of the value_date <= d set for its own day and for every later
// day, so it depresses all of them at once.
func (s *service) closingBalance(ctx context.Context, acc entity.Account, day entity.Day) (entity.Money, error) {
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day
	})
}

// closingBalanceExcludingSeq is the closing balance with one entry left out.
// The fee reversal sweep needs it: the question "is this day still overdrawn?"
// has to be asked without the fee that is itself under consideration, or the
// fee would forever justify its own existence.
func (s *service) closingBalanceExcludingSeq(
	ctx context.Context, acc entity.Account, day entity.Day, seq int,
) (entity.Money, error) {
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day && e.Seq != seq
	})
}

func (s *service) balanceWhere(
	ctx context.Context, acc entity.Account, keep entryFilter,
) (entity.Money, error) {
	entries, err := s.Entries(ctx)
	if err != nil {
		return entity.Money{}, err
	}
	total := acc.Opening
	for _, e := range entries {
		if keep(e) {
			total = total.Add(e.Amount)
		}
	}
	return total, nil
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
func (s *service) availableBalance(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	balance, err := s.closingBalance(ctx, acc, day)
	if err != nil {
		return entity.Money{}, err
	}
	return balance.Sub(s.activeHolds(acc, day)), nil
}
