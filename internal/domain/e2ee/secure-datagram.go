package e2ee

func CipherDatagram(datagram, key []byte) ([]byte, error) {
	aesgcm, err := NewAESCM(key)
	if err != nil {
		return nil, err
	}

	cipherData, err := CipherPayload(aesgcm, datagram)
	if err != nil {
		return nil, err
	}

	return cipherData, nil
}

func DecipherDatagram(datagram, key []byte) ([]byte, error) {
	aesgcm, err := NewAESCM(key)
	if err != nil {
		return nil, err
	}

	decipherData, err := DecipherPayload(aesgcm, datagram)
	if err != nil {
		return nil, err
	}

	return decipherData, nil
}
