package networking

import "github.com/kiryuhakipyatok/aloh-networking/internal/domain/e2ee"

func cipherDatagram(datagram, key []byte) ([]byte, error) {
	aesgcm, err := e2ee.NewAESCM(key)
	if err != nil {
		return nil, err
	}

	cipherData, err := e2ee.CipherPayload(aesgcm, datagram)
	if err != nil {
		return nil, err
	}

	return cipherData, nil
}

func decipherDatagram(datagram, key []byte) ([]byte, error) {
	aesgcm, err := e2ee.NewAESCM(key)
	if err != nil {
		return nil, err
	}

	decipherData, err := e2ee.DecipherPayload(aesgcm, datagram)
	if err != nil {
		return nil, err
	}

	return decipherData, nil
}
