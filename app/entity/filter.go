package entity

import "strconv"

// Filters narrow a Read. They follow the boilerplate's XFilter convention, with
// one simplification: there is no pagination, because pagination is a transport
// concern and this core has no transport.
//
// A zero value means "no constraint" throughout. Day numbering starts at 1, so
// a zero Day is unambiguous as "unset" and no pointer is needed to express it.

// AccountFilter narrows a read of accounts.
type AccountFilter struct {
	ID string
}

// LedgerEntryFilter narrows a read of journal entries.
//
// MaxValueDate is the one that matters: "every entry with value_date <= that
// day" is the brief's own definition of a closing balance, so it belongs in the
// query rather than in a loop the caller writes each time.
type LedgerEntryFilter struct {
	AccountID    string
	MaxValueDate Day
	ValueDate    Day
	// ExcludeSeq drops one entry from the result. The fee reversal sweep needs
	// it: "is this day still overdrawn?" has to be asked without the fee that is
	// itself under consideration.
	ExcludeSeq int
}

// RejectionFilter narrows a read of refused instructions.
type RejectionFilter struct {
	AccountID string
	Day       Day
}

// AuthorizationFilter narrows a read of authorizations.
type AuthorizationFilter struct {
	AuthID     string
	AccountID  string
	PostingDay Day
}

// AccrualFilter narrows a read of interest accruals.
type AccrualFilter struct {
	AccountID string
	Day       Day
}

// FeeAssessmentFilter narrows a read of fee assessments.
type FeeAssessmentFilter struct {
	AccountID string
	Day       Day
}

// EventFilter narrows a read of the event stream.
type EventFilter struct {
	AccountID  string
	PostingDay Day
}

// FeeAssessment records that an account-day has been charged an overdraft fee.
//
// It exists so the "at most once per day per account" cap is a stored row with
// an identity rather than a key in a map hidden inside a service.
type FeeAssessment struct {
	AccountID string
	Day       Day
}

// ID is the composite key, so Delete has the same one-string signature as every
// other Delete in the codebase.
func (f FeeAssessment) ID() string {
	return f.AccountID + "#" + strconv.Itoa(int(f.Day))
}
