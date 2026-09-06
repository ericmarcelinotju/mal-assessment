package apperror

import "fmt"

// AppError is a coded error. The wrapped cause is never exposed to callers
// directly; it is reachable only through errors.Unwrap.
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

func (e *AppError) Unwrap() error { return e.err }

// Is lets errors.Is match on code alone, so callers can assert the failure
// category without depending on the message text.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	return ok && t.Code == e.Code
}

func New(code ErrorCode, message string, err ...error) *AppError {
	return factory.New(code, message, err...)
}
