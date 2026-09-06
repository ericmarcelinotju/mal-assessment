// Package fee owns the overdraft fee: when it is assessed, and -- under the
// criterion-6 policy -- when it is undone.
//
// It depends on the ledger module for balances and for booking the fee entry,
// and owns only the record of which days have been charged.
package fee

import (
	"context"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

// Sentinel errors returned by the repository.
var (
	ErrNotFound      = apperror.New(apperror.ErrUnknownEventRef, "fee assessment not found")
	ErrAlreadyExists = apperror.New(apperror.ErrInvalidParameter, "fee assessment already exists")
)

type Service interface {
	Create(context.Context, entity.FeeAssessment) (entity.FeeAssessment, error)
	Read(context.Context, entity.FeeAssessmentFilter) ([]entity.FeeAssessment, error)
	Delete(context.Context, string) error

	// Assess charges any day up to the processing day whose closing balance is
	// negative and which has not been charged before.
	Assess(context.Context, entity.Account, entity.Day) error
	// ReverseOnCauseReversal undoes fees whose day is no longer negative, if the
	// configured policy allows it. A no-op under the default policy.
	ReverseOnCauseReversal(context.Context, entity.Account, entity.Day) error
}

type service struct {
	cfg    config.Config
	repo   Repository
	ledger ledger.Service
}

func NewService(cfg config.Config, repo Repository, ledgerSvc ledger.Service) Service {
	return &service{cfg: cfg, repo: repo, ledger: ledgerSvc}
}

func (s *service) Create(
	ctx context.Context, payload entity.FeeAssessment,
) (entity.FeeAssessment, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.FeeAssessment{}, apperror.New(apperror.ErrUnexpected,
			"create fee assessment error", err)
	}
	return res, nil
}

func (s *service) Read(
	ctx context.Context, filter entity.FeeAssessmentFilter,
) ([]entity.FeeAssessment, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read fee assessment error", err)
	}
	return res, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.New(apperror.ErrUnexpected, "delete fee assessment error", err)
	}
	return nil
}

// amount returns the overdraft fee denominated in the account's own currency.
//
// The brief states the fee as "AED 25.00" while ACC-002 is a BHD account. An
// AED-denominated entry cannot be booked to a BHD account without an exchange
// rate, and the brief supplies none, so the fee is 25 units of the account's
// currency. ACC-002 never goes negative, so this is not exercised by the
// canonical stream -- but the module has to have an answer, and silently
// booking AED into a BHD account would be the worse one. See NUMBERS.md.
func (s *service) amount(acc entity.Account) (entity.Money, error) {
	return entity.NewMoney(decimal.NewFromInt(s.cfg.OverdraftFeeMajor), acc.Currency)
}

// Assess sweeps every day of the window up to and including the processing day,
// charging the fee to any day whose closing ledger balance is negative and
// which has not been charged before.
//
// Two things make this a sweep rather than a single check on the current day.
//
// First, back-value postings. E7 arrives on Day 5 value-dated Day 2, so Day 2's
// closing balance becomes negative days after Day 2 ended. The fee rule is
// written in terms of "all entries with value_date <= that day", which is a
// statement about the value-dated balance, not about what was known at the
// time; the fee is therefore assessed when the ledger learns of the overdraft
// and booked with value_date equal to the day assessed.
//
// Second, and this is why criterion 2 is refutable: a back-value entry is a
// member of the value_date <= d set for its own day AND for every later day.
// Once E7 puts Day 2 at -370.00 it also puts Day 4 at -155.00 and Day 5 at
// -155.00. Accepting the Day 2 fee forces the Day 4 and Day 5 fees. There is no
// consistent reading in which E7 causes exactly one fee dated Day 2.
//
// The sweep runs ascending so that a fee booked on an earlier day is already in
// the balance when a later day is evaluated -- fees are ledger entries and
// compound like any other. On this stream the cascade changes no outcome (the
// Day 2 fee narrows Day 3 from +30.00 to +5.00, which is still positive), so
// the canonical events do not discriminate ascending-with-cascade from
// simultaneous evaluation. The synthetic cascade test covers that gap.
func (s *service) Assess(ctx context.Context, acc entity.Account, processingDay entity.Day) error {
	amount, err := s.amount(acc)
	if err != nil {
		return err
	}

	for day := entity.Day(1); day <= processingDay; day++ {
		charged, err := s.Read(ctx, entity.FeeAssessmentFilter{AccountID: acc.ID, Day: day})
		if err != nil {
			return err
		}
		if len(charged) > 0 {
			continue
		}

		balance, err := s.ledger.ClosingBalance(ctx, acc, day)
		if err != nil {
			return err
		}
		if !balance.IsNegative() {
			continue
		}

		if _, err := s.ledger.Create(ctx, entity.LedgerEntry{
			EventID:    entity.EventID("FEE-D" + strconv.Itoa(int(day))),
			AccountID:  acc.ID,
			PostingDay: processingDay,
			ValueDate:  day,
			Amount:     amount.Neg(),
			Origin:     entity.OriginOverdraftFee,
			Memo:       "overdraft fee for day " + strconv.Itoa(int(day)),
		}); err != nil {
			return err
		}
		if _, err := s.Create(ctx, entity.FeeAssessment{AccountID: acc.ID, Day: day}); err != nil {
			return err
		}
	}
	return nil
}

// ReverseOnCauseReversal is the acceptance-criterion-6 branch, active only
// under LEDGER_FEE_REVERSAL_POLICY=on_cause_reversal.
//
// Criterion 6 claims that after E9 all balances and fees return to their pre-E7
// values. Under the brief as written that is false: the brief grants an
// assessment primitive and no de-assessment primitive, so the three fees stand
// and the account is permanently 75.10 short of where it would have been.
//
// The criterion is not, however, arithmetically wrong -- it fails on a missing
// rule. Adding that rule does not violate append-only: a fee is reversed by
// appending a contra entry, never by touching the original. This function is
// that rule, so the argument in REJECTED.md can be executed rather than merely
// asserted. It is off by default because defaulting to my own extension would
// quietly overwrite the specification.
//
// A fee is reversed when its day is no longer negative once that fee itself is
// excluded. Excluding it is essential: a fee makes its own day more negative,
// so a fee evaluated inclusively would forever justify its own existence.
//
// Oscillation is impossible by construction, which is why the loop is safe.
// Assessment tests the fee-INCLUSIVE balance and requires it negative; reversal
// tests the fee-EXCLUSIVE balance and requires it non-negative. A day satisfying
// the reversal test has a non-negative balance without its fee, so it would not
// have been assessed in that state. The iteration cap guards against a future
// rule change breaking that property, not against this one.
func (s *service) ReverseOnCauseReversal(
	ctx context.Context, acc entity.Account, processingDay entity.Day,
) error {
	if s.cfg.FeeReversal != config.FeeReversalOnCauseReversal {
		return nil
	}

	for iter := 0; ; iter++ {
		if iter > s.cfg.MaxSweepIterations {
			return apperror.New(apperror.ErrNoConvergence,
				"fee reversal sweep did not converge for "+acc.ID)
		}

		entries, err := s.ledger.Read(ctx, entity.LedgerEntryFilter{AccountID: acc.ID})
		if err != nil {
			return err
		}
		reversed := make(map[int]bool, len(entries))
		for _, e := range entries {
			if e.IsFeeReversal() {
				reversed[e.ReversesSeq] = true
			}
		}

		progressed := false
		for _, charge := range entries {
			if !charge.IsFee() || reversed[charge.Seq] {
				continue
			}
			without, err := s.ledger.ClosingBalanceExcluding(ctx, acc, charge.ValueDate, charge.Seq)
			if err != nil {
				return err
			}
			if without.IsNegative() {
				continue
			}
			if _, err := s.ledger.Create(ctx, entity.LedgerEntry{
				EventID:     charge.EventID,
				AccountID:   acc.ID,
				PostingDay:  processingDay,
				ValueDate:   charge.ValueDate,
				Amount:      charge.Amount.Neg(),
				Origin:      entity.OriginFeeReversal,
				Memo:        "reversal of overdraft fee for day " + strconv.Itoa(int(charge.ValueDate)),
				ReversesSeq: charge.Seq,
			}); err != nil {
				return err
			}
			assessment := entity.FeeAssessment{AccountID: acc.ID, Day: charge.ValueDate}
			if err := s.Delete(ctx, assessment.ID()); err != nil {
				return err
			}
			progressed = true
		}

		if !progressed {
			return nil
		}
	}
}
