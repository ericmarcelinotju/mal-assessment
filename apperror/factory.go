package apperror

// ErrorFactory stamps a service prefix onto every code so that codes are
// globally unique once several services are aggregated.
type ErrorFactory struct {
	prefix string
}

// factory is initialised eagerly. The upstream boilerplate left this nil until
// Init was called and panicked on the first New; see REJECTED.md.
var factory = &ErrorFactory{prefix: ""}

func Init(prefix string) { factory = &ErrorFactory{prefix: prefix} }

func (f *ErrorFactory) New(code ErrorCode, message string, err ...error) *AppError {
	appError := &AppError{Code: ErrorCode(f.prefix + string(code)), Message: message}
	if len(err) > 0 {
		appError.err = err[0]
	}
	return appError
}
