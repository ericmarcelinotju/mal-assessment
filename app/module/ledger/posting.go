package ledger

import (
	"strconv"

	"github.com/ericmarcelinotju/mal-assessment/app/entity"
	"github.com/ericmarcelinotju/mal-assessment/apperror"
)

// Post applies one instruction to the ledger.
func (s *service) Post(ev entity.Event) ([]entity.LedgerEntry, error) {
	acc, err := s.account(ev.AccountID)
	if err != nil {
		return nil, s.reject(ev, apperror.ErrUnknownAccount, "unknown account "+ev.AccountID)
	}
	if ev.HasStatedAmount() && ev.Amount.Currency() != acc.Currency {
		return nil, s.reject(ev, apperror.ErrCurrencyMismatch,
			"event is "+ev.Amount.Currency().String()+" but account "+acc.ID+" is "+acc.Currency.String())
	}

	switch ev.Type {
	case entity.EventCredit:
		return s.postAmount(acc, ev, ev.Amount, entity.OriginInstruction)
	case entity.EventDebit:
		return s.postAmount(acc, ev, ev.Amount.Neg(), entity.OriginInstruction)
	case entity.EventAuthorization:
		return nil, s.authorize(acc, ev)
	case entity.EventSettlement:
		return s.settle(acc, ev)
	case entity.EventReversal:
		return s.reverse(acc, ev)
	default:
		return nil, s.reject(ev, apperror.ErrInvalidParameter, "unsupported event type "+string(ev.Type))
	}
}

// postAmount books a signed amount, splitting it into instalments when asked.
//
// Note what is absent: a credit or a debit is not subjected to the
// available-balance test. The brief applies that test to authorizations only,
// and rightly so -- a debit that cannot be funded is exactly the situation the
// overdraft fee exists to price. Refusing it would mean the fee rule could
// never fire at all.
func (s *service) postAmount(
	acc entity.Account, ev entity.Event, signed entity.Money, origin entity.EntryOrigin,
) ([]entity.LedgerEntry, error) {
	parts := []entity.Money{signed}
	if ev.Instalments > 1 {
		var err error
		parts, err = SplitInstalments(signed, ev.Instalments)
		if err != nil {
			return nil, s.reject(ev, apperror.ErrInvalidParameter, err.Error())
		}
	}

	out := make([]entity.LedgerEntry, 0, len(parts))
	for i, p := range parts {
		memo := string(ev.Type)
		if len(parts) > 1 {
			memo = memo + " instalment " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(parts))
		}
		out = append(out, s.log.append(entity.LedgerEntry{
			EventID:    ev.ID,
			AccountID:  acc.ID,
			PostingDay: ev.PostingDay,
			ValueDate:  ev.ValueDate,
			Amount:     p,
			Origin:     origin,
			Memo:       memo,
		}))
	}
	return out, nil
}
