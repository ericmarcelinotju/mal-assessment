package entity

// AuthState is the lifecycle of an authorization.
type AuthState string

const (
	// AuthApproved: the hold is active and reduces available balance.
	AuthApproved AuthState = "APPROVED"
	// AuthDeclined: the availability test failed. No hold, no entry, and the
	// authorization is retained only so the report can explain the decline.
	AuthDeclined AuthState = "DECLINED"
	// AuthSettled: a settlement consumed it. The hold is released in full.
	AuthSettled AuthState = "SETTLED"
)

// Authorization is a hold placed on an account. A hold never moves the ledger
// balance; it reduces available balance only. The ledger moves at settlement.
type Authorization struct {
	AuthID      string
	AccountID   string
	EventID     EventID
	PostingDay  Day
	ValueDate   Day
	Hold        Money
	State       AuthState
	DeclineNote string

	// SettledDay and SettledAmount are set when a settlement consumes the hold.
	SettledDay    Day
	SettledAmount Money
}

// IsActive reports whether the hold still reduces available balance.
func (a Authorization) IsActive() bool { return a.State == AuthApproved }
