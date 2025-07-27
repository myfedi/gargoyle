package domainerrors

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrBadRequest   = errors.New("bad request")
	ErrInternal     = errors.New("internal server error")
)

// DomainError wraps errors with more context about the resource state to pass
// this information down without being dependant on one implementation
// (like fiber.NewError(fiber.StatusBadRequest, ...)), which isn't allowed to be
// used in the domain package.
// So we need the error cause and the error to pass it downstream.
// See infrastructure/web/error_handler.go on how this is resolved to an HTTP status code
type DomainError struct {
	Code        error  // one of the base errors above
	Message     string // optional human-readable detail
	ParentError *error // if applicable, the error that caused the execution to fail
}

func (e *DomainError) Error() string {
	return e.Code.Error() + ": " + e.Message
}

func New(code error, msg string) *DomainError {
	return &DomainError{Code: code, Message: msg}
}

func NewErr(code error, err error) *DomainError {
	return &DomainError{Code: code, Message: err.Error(), ParentError: &err}
}
