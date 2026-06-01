package e2ee

import (
	"crypto/cipher"
	"io"

	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
)

type quicStream any

type SecureStream[T quicStream] struct {
	quicStream T
	aead       cipher.AEAD
}

func NewSecureStream[T quicStream](quicStream T, key []byte) (*SecureStream[T], error) {
	aesgcm, err := NewAESCM(key)
	if err != nil {
		return nil, err
	}

	return &SecureStream[T]{
		quicStream: quicStream,
		aead:       aesgcm,
	}, nil
}

func (ss *SecureStream[T]) Read(payload []byte) (n int, err error) {
	stream, ok := any(ss.quicStream).(io.Reader)
	if !ok {
		return 0, errs.ErrInvalidType
	}
	return stream.Read(payload)
}

func (ss *SecureStream[T]) Write(payload []byte) (n int, err error) {
	stream, ok := any(ss.quicStream).(io.Writer)
	if !ok {
		return 0, errs.ErrInvalidType
	}
	return stream.Write(payload)
}

func (ss *SecureStream[T]) Send(payload []byte) error {
	cipherPayload, err := CipherPayload(ss.aead, payload)
	if err != nil {
		return err
	}
	stream, ok := any(ss.quicStream).(io.Writer)
	if !ok {
		return errs.ErrInvalidType
	}

	if _, err = stream.Write(cipherPayload); err != nil {
		return err
	}

	return nil
}

func (ss *SecureStream[T]) Close() error {
	stream, ok := any(ss.quicStream).(io.Closer)
	if !ok {
		return errs.ErrInvalidType
	}
	if err := stream.Close(); err != nil {
		return err
	}
	return nil
}

func (ss *SecureStream[T]) Receive() ([]byte, error) {
	stream, ok := any(ss.quicStream).(io.Reader)
	if !ok {
		return nil, errs.ErrInvalidType
	}
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	return DecipherPayload(ss.aead, data)
}
