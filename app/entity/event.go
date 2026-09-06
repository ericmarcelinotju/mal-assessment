package entity

// Day is a day number inside the replay window, 1..6. The window is short and
// closed, so a day is an ordinal rather than a calendar date; nothing in the
// engine depends on wall-clock time, which is what makes the replay
// deterministic.
type Day int

// EventID is the identifier from the brief: E1..E10.
type EventID string

// EventType enumerates what the outside world can tell the ledger.
type EventType string

const (
	EventCredit        EventType = "CREDIT"
	EventDebit         EventType = "DEBIT"
	EventAuthorization EventType = "AUTHORIZATION"
	EventSettlement    EventType = "SETTLEMENT"
	EventReversal      EventType = "REVERSAL"
)

// Event is an instruction received by the ledger. It is not itself a ledger
// entry: an authorization that is declined, or a settlement with no matching
// authorization, produces an event record and no entry at all.
//
// Two clocks, which is the whole difficulty of this exercise:
//
//	PostingDay -- when the ledger learned of the event. Monotonic, and the
//	              order in which events are replayed.
//	ValueDate  -- when the event economically applies. May be in the past.
//
// E7 arrives on Day 5 carrying value date Day 2. That is a back-value posting:
// legitimate and routine in core banking, and the reason a day's closing
// balance has to be qualified by the day it is observed from.
type Event struct {
	ID         EventID
	Type       EventType
	PostingDay Day
	ValueDate  Day
	AccountID  string

	// Amount is the instructed amount. For an authorization it is the hold; for
	// a settlement it is the settled amount, which may differ from the hold.
	Amount Money

	// AuthID names the authorization for AUTHORIZATION and SETTLEMENT events.
	AuthID string

	// ReversesEventID names the event a REVERSAL undoes.
	ReversesEventID EventID

	// Instalments splits the amount into n postings. Zero and one both mean a
	// single posting.
	Instalments int
}

// HasStatedAmount reports whether the event carries an amount of its own that
// must agree with the account currency.
//
// A REVERSAL deliberately does not. Its amount is derived from the entries it
// reverses, which is the only way a reversal is guaranteed to undo exactly what
// was done -- restating the amount in the reversal instruction would let a typo
// leave a residue behind, and a residue on a reversal stays invisible until it
// is expensive. So a reversal is monetary in effect while carrying no stated
// amount to validate.
func (e Event) HasStatedAmount() bool {
	switch e.Type {
	case EventCredit, EventDebit, EventSettlement, EventAuthorization:
		return true
	default:
		return false
	}
}
