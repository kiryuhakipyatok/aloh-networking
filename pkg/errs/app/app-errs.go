package errs

import (
	"errors"
	"fmt"
)

var (
	ErrNotFoundBase            = errors.New("not found")
	ErrAlreadyExistsBase       = errors.New("already exists")
	ErrRequestTimeoutBase      = errors.New("request timeout")
	ErrInvalidJsonBase         = errors.New("invalid json")
	ErrValidationBase          = errors.New("valdiation error")
	ErrInternalServerErrorBase = errors.New("interanl error")
	AppClosingBase             = errors.New("app closing")
	ErrInvalidType             = errors.New("invalid type")
)

type AppError struct {
	Op  string
	Err error
}

func (ae AppError) Error() string {
	return ae.Err.Error()
}

func (ae AppError) Unwrap() error {
	return ae.Err
}

func NewAppError(op string, err error) AppError {
	return AppError{
		Op:  op,
		Err: err,
	}
}

func ErrNotFound(op string) AppError {
	return AppError{Op: op, Err: ErrNotFoundBase}
}

func ErrAlreadyExists(op string) AppError {
	return AppError{Op: op, Err: ErrAlreadyExistsBase}
}

func ErrRequestTimeout(op string) AppError {
	return AppError{Op: op, Err: ErrRequestTimeoutBase}
}

func ErrInvalidJson(op string, err error) AppError {
	return AppError{Op: op, Err: fmt.Errorf("%w : %w", ErrInvalidJsonBase, err)}
}

func ErrValidation(op string, err error) AppError {
	return AppError{Op: op, Err: fmt.Errorf("%w : %w", ErrValidationBase, err)}
}

func ErrInternal(op string) AppError {
	return AppError{Op: op, Err: ErrInternalServerErrorBase}
}

func AppClosing(op string) AppError {
	return AppError{Op: op, Err: AppClosingBase}
}
