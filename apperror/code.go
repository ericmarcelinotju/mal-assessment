package apperror

type ErrorCode string

const (
	ErrUnexpected ErrorCode = "0000"

	// 1xxx: General Request Errors
	ErrInvalidParameter ErrorCode = "1000"
	ErrMissingParameter ErrorCode = "1001"

	// 2xxx: Validation Errors (service-specific)
	ErrCurrencyMismatch     ErrorCode = "2000"
	ErrUnknownAccount       ErrorCode = "2001"
	ErrOrphanSettlement     ErrorCode = "2002"
	ErrDuplicateAuthID      ErrorCode = "2003"
	ErrAuthNotActive        ErrorCode = "2004"
	ErrInsufficientFunds    ErrorCode = "2005"
	ErrUnknownEventRef      ErrorCode = "2006"
	ErrAlreadyReversed      ErrorCode = "2007"
	ErrValueDateOutOfWindow ErrorCode = "2008"
	ErrNegativeAmount       ErrorCode = "2009"

	// 3xxx: Internal Errors
	ErrInternalValidation ErrorCode = "3005"
	ErrNoConvergence      ErrorCode = "3006"
)
