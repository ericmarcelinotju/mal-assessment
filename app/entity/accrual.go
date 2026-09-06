package entity

// Accrual is one interest accrual record for one account on one day.
//
// Accruals are append-only in the same sense entries are. When a back-value
// posting changes a day's closing balance after that day's accrual was already
// booked, the earlier record is left untouched and a further record is appended
// carrying the difference. The net accrual for a day is the sum of its records.
//
// Worked example from the canonical stream, ACC-001 day 2:
//
//	+0.10  booked at the day 2 close, balance  250.00
//	-0.10  adjustment at the day 5 close, balance -395.00 (E7 arrived back-valued)
//	+0.09  adjustment at the day 6 close, balance  225.00 (E9 reversed E7)
//	----- net 0.09
//
// This is a back-value interest adjustment, which is what a core banking system
// does; the alternative -- freezing each day's accrual at its own close -- is
// simpler but leaves the accrual subledger disagreeing with the value-dated
// balances it claims to describe. See AMBIGUITIES.md.
type Accrual struct {
	Seq          int
	AccountID    string
	Day          Day // the day the interest is for
	BookedOnDay  Day // the processing day this record was appended
	Amount       Money
	Basis        Money // the closing balance the accrual was computed on
	IsAdjustment bool
}
