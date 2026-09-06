package entity

import "strconv"

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
