// Package apperror provides coded errors. The code is what the day report
// prints beside a rejected instruction -- "[2002] settlement for Auth-Z has no
// preceding authorization" -- so a reader can tell one class of refusal from
// another without parsing prose.
//
// There is deliberately no Unwrap or Is here. Nothing in this ledger inspects
// an error programmatically: rejections are recorded on the log as data and
// reported under their own day, so the only consumer of an error value is a
// person reading the report. Implementing the errors.Is contract for a caller
// that does not exist would be scaffolding.
package apperror

import "fmt"

// AppError is a coded error. A wrapped cause, where one is supplied, appears in
// the formatted message and nowhere else.
type AppError struct {
	Code    ErrorCode
	Message string
	err     error
}

func (e *AppError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func New(code ErrorCode, message string, err ...error) *AppError {
	appError := &AppError{Code: code, Message: message}
	if len(err) > 0 {
		appError.err = err[0]
	}
	return appError
}
