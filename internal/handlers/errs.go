package handlers

import (
	"errors"
	"fmt"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/errs"
)

const (
	SUCCESS = iota
	NOT_FOUND
	ALREADY_EXISTS
	REQUEST_TIMEOUT
	VALIDATION_ERROR
	CHAT_ERROR
	VOICE_ERROR
	VIDEO_ERROR
	INTERNAL_ERROR
)

type ErrorCode struct {
	Code uint
}

func (rm ErrorCode) Error() string {
	return fmt.Sprintf("%d", rm.Code)
}

func NewErrorMessage(code uint) ErrorCode {
	return ErrorCode{
		Code: code,
	}
}

func chatErrorMessage() ErrorCode {
	return NewErrorMessage(CHAT_ERROR)
}

func voiceErrorMessage() ErrorCode {
	return NewErrorMessage(VOICE_ERROR)
}

func videoErrorMessage() ErrorCode {
	return NewErrorMessage(VIDEO_ERROR)
}

func internalServerErrorMessage() ErrorCode {
	return NewErrorMessage(INTERNAL_ERROR)
}

func errorNotFoundMessage() ErrorCode {
	return NewErrorMessage(NOT_FOUND)
}

func errorAlreadyExistsMessage() ErrorCode {
	return NewErrorMessage(ALREADY_EXISTS)
}

func errorRequestTimeoutMessage() ErrorCode {
	return NewErrorMessage(REQUEST_TIMEOUT)
}

func errorValidationMessage() ErrorCode {
	return NewErrorMessage(VALIDATION_ERROR)
}

func processError(err error) error {
	switch {
	case errors.Is(err, errs.ErrAlreadyExistsBase):
		return errorAlreadyExistsMessage()
	case errors.Is(err, errs.ErrNotFoundBase):
		return errorNotFoundMessage()
	case errors.Is(err, errs.ErrRequestTimeoutBase):
		return errorRequestTimeoutMessage()
	case errors.Is(err, errs.ErrValidationBase):
		return errorValidationMessage()
	default:
		return internalServerErrorMessage()
	}
}
