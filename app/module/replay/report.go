package replay

import (
	"context"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
)

// Report builds one row per day per account by asking each module for the part
// it owns: balances from ledger, holds from authorization, accruals from
// interest, rejections from the journal.
//
// Everything here is derived on demand. No figure is cached at close time, so
// the report cannot drift from the entries that justify it.
func (s *service) Report(ctx context.Context) ([]entity.DayReport, error) {
	accounts, err := s.accounts.All(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.ledger.Entries(ctx)
	if err != nil {
		return nil, err
	}
	journalErrors, err := s.ledger.Errors(ctx)
	if err != nil {
		return nil, err
	}
	accruals, err := s.interest.Accruals(ctx)
	if err != nil {
		return nil, err
	}
	auths, err := s.auths.All(ctx)
	if err != nil {
		return nil, err
	}

	var out []entity.DayReport
	for day := entity.Day(1); day <= entity.Day(s.cfg.WindowDays); day++ {
		for _, acc := range accounts {
			closing, err := s.ledger.ClosingBalance(ctx, acc, day)
			if err != nil {
				return nil, err
			}
			holds, err := s.auths.ActiveHolds(ctx, acc, day)
			if err != nil {
				return nil, err
			}
			available, err := s.auths.AvailableBalance(ctx, acc, day)
			if err != nil {
				return nil, err
			}
			accrued, err := s.interest.NetAccrual(ctx, acc, day)
			if err != nil {
				return nil, err
			}

			row := entity.DayReport{
				Day:              day,
				AccountID:        acc.ID,
				Currency:         acc.Currency,
				ClosingBalance:   closing,
				ActiveHolds:      holds,
				AvailableBalance: available,
				InterestAccrued:  accrued,
			}

			for _, e := range entries {
				if e.AccountID != acc.ID || e.ValueDate != day {
					continue
				}
				switch e.Origin {
				case entity.OriginOverdraftFee:
					row.FeesAssessed = append(row.FeesAssessed, e)
				case entity.OriginFeeReversal:
					row.FeesReversed = append(row.FeesReversed, e)
				case entity.OriginCapitalisation:
					credit := e
					row.Capitalisation = &credit
				}
				if e.Origin != entity.OriginCapitalisation {
					row.Entries = append(row.Entries, e)
				}
			}
			for _, a := range accruals {
				if a.AccountID == acc.ID && a.Day == day {
					row.AccrualTrail = append(row.AccrualTrail, a)
				}
			}
			for _, a := range auths {
				if a.AccountID == acc.ID && a.PostingDay == day {
					row.Authorizations = append(row.Authorizations, a)
				}
			}
			for _, e := range journalErrors {
				if e.AccountID == acc.ID && e.Day == day {
					row.Errors = append(row.Errors, e)
				}
			}

			out = append(out, row)
		}
	}
	return out, nil
}
