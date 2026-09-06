// Package replay orchestrates the other modules over the six-day window: it
// routes each instruction to the module that owns it, runs the end-of-day
// cycle in the right order, and assembles the report.
//
// It is the only module that depends on all the others, and nothing depends on
// it. That is what keeps the graph a DAG: account, ledger, rejection,
// authorization, fee and interest never need to know a replay exists.
package replay

import (
	"context"
	"sort"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/app/module/account"
	"github.com/ericmarcelinotju/mal-assessment/app/module/authorization"
	"github.com/ericmarcelinotju/mal-assessment/app/module/fee"
	"github.com/ericmarcelinotju/mal-assessment/app/module/interest"
	"github.com/ericmarcelinotju/mal-assessment/app/module/ledger"
	"github.com/ericmarcelinotju/mal-assessment/app/module/rejection"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
	"github.com/ericmarcelinotju/mal-assessment/config"
)

type Service interface {
	Create(context.Context, entity.Event) (entity.Event, error)
	Read(context.Context, entity.EventFilter) ([]entity.Event, error)

	// Post applies one instruction, routing it to the owning module.
	Post(context.Context, entity.Event) ([]entity.LedgerEntry, error)
	// CloseDay runs the end-of-day cycle for every account.
	CloseDay(context.Context, entity.Day) error
	// Run loads a stream, posts every event in order and closes every day.
	Run(context.Context, []entity.Event) error
	// Report returns one row per day per account, plus the audit trail.
	Report(context.Context) ([]entity.DayReport, error)
	// Config reports the active configuration, so a caller rendering the report
	// can state which fee-reversal policy produced the numbers.
	Config() config.Config
}

type service struct {
	cfg config.Config

	repo       Repository
	accounts   account.Service
	ledger     ledger.Service
	rejections rejection.Service
	auths      authorization.Service
	fees       fee.Service
	interest   interest.Service
}

func NewService(
	cfg config.Config,
	repo Repository,
	accountSvc account.Service,
	ledgerSvc ledger.Service,
	rejectionSvc rejection.Service,
	authSvc authorization.Service,
	feeSvc fee.Service,
	interestSvc interest.Service,
) Service {
	return &service{
		cfg:        cfg,
		repo:       repo,
		accounts:   accountSvc,
		ledger:     ledgerSvc,
		rejections: rejectionSvc,
		auths:      authSvc,
		fees:       feeSvc,
		interest:   interestSvc,
	}
}

func (s *service) Config() config.Config { return s.cfg }

func (s *service) Create(ctx context.Context, payload entity.Event) (entity.Event, error) {
	res, err := s.repo.Create(ctx, payload)
	if err != nil {
		return entity.Event{}, apperror.New(apperror.ErrUnexpected, "create event error", err)
	}
	return res, nil
}

func (s *service) Read(ctx context.Context, filter entity.EventFilter) ([]entity.Event, error) {
	res, err := s.repo.Read(ctx, filter)
	if err != nil {
		return nil, apperror.New(apperror.ErrUnexpected, "read event error", err)
	}
	return res, nil
}

// Post routes one instruction to the module that owns it. It decides nothing
// about the instruction itself beyond which module should see it.
func (s *service) Post(ctx context.Context, ev entity.Event) ([]entity.LedgerEntry, error) {
	accounts, err := s.accounts.Read(ctx, entity.AccountFilter{ID: ev.AccountID})
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrUnknownAccount,
			"unknown account "+ev.AccountID)
	}
	acc := accounts[0]

	if ev.HasStatedAmount() && ev.Amount.Currency() != acc.Currency {
		return nil, s.rejections.Create(ctx, ev, apperror.ErrCurrencyMismatch,
			"event is "+ev.Amount.Currency().String()+" but account "+acc.ID+
				" is "+acc.Currency.String())
	}

	switch ev.Type {
	case entity.EventCredit:
		return s.ledger.Post(ctx, acc, ev, ev.Amount, entity.OriginInstruction)
	case entity.EventDebit:
		return s.ledger.Post(ctx, acc, ev, ev.Amount.Neg(), entity.OriginInstruction)
	case entity.EventAuthorization:
		return nil, s.auths.Authorize(ctx, acc, ev)
	case entity.EventSettlement:
		return s.auths.Settle(ctx, acc, ev)
	case entity.EventReversal:
		return s.ledger.Reverse(ctx, acc, ev)
	default:
		return nil, s.rejections.Create(ctx, ev, apperror.ErrInvalidParameter,
			"unsupported event type "+string(ev.Type))
	}
}

// Run loads a stream and replays it across the whole window.
//
// Events are ordered by posting day, then by their position in the supplied
// stream. This matters because the brief lists E9 (posting day 6) before E10
// (posting day 5), so "replayed in this order" and "replayed in time order"
// disagree. Posting day wins: a ledger cannot learn of a Day 6 event before a
// Day 5 one. Within a single posting day the listed order is preserved, so the
// brief's sequencing is honoured wherever it is not self-contradictory. The two
// events involved touch different accounts, so on this stream the choice
// changes nothing -- but a replay engine that silently depends on input order is
// one that produces different answers for the same facts. See AMBIGUITIES.md.
//
// Rejected instructions do not abort the run. A declined authorization and an
// orphan settlement are recorded outcomes, not failures, and the remaining
// events still have to be processed.
func (s *service) Run(ctx context.Context, events []entity.Event) error {
	for _, ev := range events {
		if _, err := s.Create(ctx, ev); err != nil {
			return err
		}
	}
	ordered, err := s.Read(ctx, entity.EventFilter{})
	if err != nil {
		return err
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].PostingDay < ordered[j].PostingDay
	})

	next := 0
	for day := entity.Day(1); day <= entity.Day(s.cfg.WindowDays); day++ {
		for next < len(ordered) && ordered[next].PostingDay == day {
			//nolint:errcheck // rejections are recorded and reported under their own day
			_, _ = s.Post(ctx, ordered[next])
			next++
		}
		if err := s.CloseDay(ctx, day); err != nil {
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
func (s *service) CloseDay(ctx context.Context, day entity.Day) error {
	accounts, err := s.accounts.Read(ctx, entity.AccountFilter{})
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if err := s.fees.Assess(ctx, acc, day); err != nil {
			return err
		}
		if err := s.fees.ReverseOnCauseReversal(ctx, acc, day); err != nil {
			return err
		}
		// The reversal sweep changes balances, so interest is accrued against
		// the post-sweep position.
		if err := s.interest.Accrue(ctx, acc, day); err != nil {
			return err
		}
		if day == entity.Day(s.cfg.CapitalisationDay) {
			if err := s.interest.Capitalise(ctx, acc, day); err != nil {
				return err
			}
		}
	}
	return nil
}

// Report builds one row per day per account by asking each module for the part
// it owns: balances from ledger, holds from authorization, accruals from
// interest, refusals from rejection.
//
// Everything here is derived on demand. No figure is cached at close time, so
// the report cannot drift from the entries that justify it.
func (s *service) Report(ctx context.Context) ([]entity.DayReport, error) {
	accounts, err := s.accounts.Read(ctx, entity.AccountFilter{})
	if err != nil {
		return nil, err
	}

	var out []entity.DayReport
	for day := entity.Day(1); day <= entity.Day(s.cfg.WindowDays); day++ {
		for _, acc := range accounts {
			row, err := s.reportRow(ctx, acc, day)
			if err != nil {
				return nil, err
			}
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *service) reportRow(
	ctx context.Context, acc entity.Account, day entity.Day,
) (entity.DayReport, error) {
	closing, err := s.ledger.ClosingBalance(ctx, acc, day)
	if err != nil {
		return entity.DayReport{}, err
	}
	holds, err := s.auths.ActiveHolds(ctx, acc, day)
	if err != nil {
		return entity.DayReport{}, err
	}
	available, err := s.auths.AvailableBalance(ctx, acc, day)
	if err != nil {
		return entity.DayReport{}, err
	}
	accrued, err := s.interest.NetAccrual(ctx, acc, day)
	if err != nil {
		return entity.DayReport{}, err
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

	entries, err := s.ledger.Read(ctx, entity.LedgerEntryFilter{AccountID: acc.ID, ValueDate: day})
	if err != nil {
		return entity.DayReport{}, err
	}
	for _, e := range entries {
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

	row.AccrualTrail, err = s.interest.Read(ctx,
		entity.AccrualFilter{AccountID: acc.ID, Day: day})
	if err != nil {
		return entity.DayReport{}, err
	}
	row.Authorizations, err = s.auths.Read(ctx,
		entity.AuthorizationFilter{AccountID: acc.ID, PostingDay: day})
	if err != nil {
		return entity.DayReport{}, err
	}
	row.Errors, err = s.rejections.Read(ctx,
		entity.RejectionFilter{AccountID: acc.ID, Day: day})
	if err != nil {
		return entity.DayReport{}, err
	}

	return row, nil
}
