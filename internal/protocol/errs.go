package protocol

import (
	"errors"
	"fmt"
	"networking/pkg/errs"

	"github.com/quic-go/quic-go"
)

var (
	chatError       = "failed to write msg in chat"
	voiceError      = "failed to send voice data"
	videoError      = "failed to send video data"
	internalError   = "internal server error"
	notFound        = "not found"
	alreadyExists   = "already exists"
	requestTimeout  = "request timeout"
	validationError = "validaion error"
)

type ErrorMessage struct {
	Code int8   `json:"code"`
	Msg  string `json:"msg"`
}

func (rm ErrorMessage) Error() string {
	return fmt.Sprintf("code: %d, msg: %v", rm.Code, rm.Msg)
}

func NewErrorMessage(msg string, code int8) ErrorMessage {
	return ErrorMessage{
		Code: code,
		Msg:  msg,
	}
}

func chatErrorMessage() ErrorMessage {
	return NewErrorMessage(chatError, int8(quic.InternalError))
}

func voiceErrorMessage() ErrorMessage {
	return NewErrorMessage(voiceError, int8(quic.InternalError))
}

func videoErrorMessage() ErrorMessage {
	return NewErrorMessage(videoError, int8(quic.InternalError))
}

func internalServerErrorMessage() ErrorMessage {
	return NewErrorMessage(internalError, int8(quic.InternalError))
}

func errorNotFoundMessage() ErrorMessage {
	return NewErrorMessage(notFound, int8(quic.ProtocolViolation))
}

func errorAlreadyExistsMessage() ErrorMessage {
	return NewErrorMessage(alreadyExists, int8(quic.ProtocolViolation))
}

func errorRequestTimeoutMessage() ErrorMessage {
	return NewErrorMessage(requestTimeout, int8(quic.InternalError))
}

func errorValidationMessage() ErrorMessage {
	return NewErrorMessage(validationError, int8(quic.ProtocolViolation))
}

func ProcessError(err error) error {
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
