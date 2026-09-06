package entity

// LedgerError is a rejected instruction. Rejections are recorded, not
// discarded: "no entry was made" is itself a fact the day's report has to
// state, and an operator needs to know why.
type LedgerError struct {
	Day       Day
	EventID   EventID
	AccountID string
	Code      string
	Message   string
}

// DayReport is one account's position at one day's close, as observed at the
// end of the replay. Every figure here is derived; nothing is stored twice.
type DayReport struct {
	Day       Day
	AccountID string
	Currency  Currency

	// ClosingBalance sums every entry with value_date <= Day.
	ClosingBalance Money
	// ActiveHolds is the sum of holds still outstanding at this day.
	ActiveHolds Money
	// AvailableBalance is ClosingBalance - ActiveHolds.
	AvailableBalance Money

	// FeesAssessed lists overdraft fees value-dated to this day, and
	// FeesReversed the contra entries against them.
	FeesAssessed []LedgerEntry
	FeesReversed []LedgerEntry

	// InterestAccrued is the net accrual for this day, and AccrualTrail the
	// append-only records that sum to it.
	InterestAccrued Money
	AccrualTrail    []Accrual

	// Authorizations are the auth events whose posting day is this day, with
	// their final state.
	Authorizations []Authorization

	// Entries are the entries value-dated to this day.
	Entries []LedgerEntry

	// Errors are the instructions rejected on this day.
	Errors []LedgerError

	// Capitalisation is the single interest credit, set only on the
	// capitalisation day.
	Capitalisation *LedgerEntry
}
