package apperror

type ErrorCode string

const (
	// 1xxx: general request errors
	ErrUnexpected       ErrorCode = "0000"
	ErrInvalidParameter ErrorCode = "1000"

	// 2xxx: validation errors, specific to this service
	ErrCurrencyMismatch  ErrorCode = "2000"
	ErrUnknownAccount    ErrorCode = "2001"
	ErrOrphanSettlement  ErrorCode = "2002"
	ErrDuplicateAuthID   ErrorCode = "2003"
	ErrAuthNotActive     ErrorCode = "2004"
	ErrInsufficientFunds ErrorCode = "2005"
	ErrUnknownEventRef   ErrorCode = "2006"
	ErrAlreadyReversed   ErrorCode = "2007"

	// 3xxx: internal errors
	ErrInternalValidation ErrorCode = "3005"
	ErrNoConvergence      ErrorCode = "3006"
)
