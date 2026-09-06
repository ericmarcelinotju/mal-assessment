package ledger

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// entryFilter selects which entries participate in a balance. Every balance in
// this module is "the sum of the entries that pass a filter", which keeps the
// variants the rules demand from drifting apart.
type entryFilter func(entity.LedgerEntry) bool

// ClosingBalance is the closing ledger balance for a day: the opening balance
// plus every entry with value_date <= day.
//
// This is the definition the brief gives in parentheses, and it is the reason
// criterion 2 cannot be satisfied. A back-value entry does not land on one day;
// it is a member of the value_date <= d set for its own day and for every later
// day, so it depresses all of them at once.
func (s *service) ClosingBalance(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.Money, error) {
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day
	})
}

// ClosingBalanceExcluding is the closing balance with one entry left out.
// The fee reversal sweep needs it: the question "is this day still overdrawn?"
// has to be asked without the fee that is itself under consideration, or the
// fee would forever justify its own existence.
func (s *service) ClosingBalanceExcluding(
	ctx context.Context, acc entity.Account, day entity.Day, seq int,
) (entity.Money, error) {
	return s.balanceWhere(ctx, acc, func(e entity.LedgerEntry) bool {
		return e.AccountID == acc.ID && e.ValueDate <= day && e.Seq != seq
	})
}

func (s *service) balanceWhere(
	ctx context.Context, acc entity.Account, keep entryFilter,
) (entity.Money, error) {
	all, err := s.Entries(ctx)
	if err != nil {
		return entity.Money{}, err
	}
	total := acc.Opening
	for _, e := range all {
		if keep(e) {
			total = total.Add(e.Amount)
		}
	}
	return total, nil
}
