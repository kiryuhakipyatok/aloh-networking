package utils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"math/big"

	"github.com/quic-go/quic-go"
)

func Uint8ToPtr(ui uint8) *uint8 {
	return &ui
}

func GenerateTLSConfig(protos []string) *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{tlsCert},
		NextProtos:         protos,
	}
}

func SetFirstByte(fb byte, data []byte) []byte {
	sdp := make([]byte, 0, len(data)+1)
	sdp = append(sdp, fb)
	sdp = append(sdp, data...)
	return sdp
}

func CheckErr(ctx context.Context, err error) error {

	if errors.Is(err, context.Canceled) || ctx.Err() != nil || errors.Is(err, io.EOF) {
		return nil
	}

	var appErr *quic.ApplicationError
	if errors.As(err, &appErr) {
		if appErr.ErrorCode == 0 {
			return nil
		}
	}

	return err

}
