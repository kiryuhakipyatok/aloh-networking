package e2ee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

func NewKeys() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pubKey := privKey.PublicKey()
	return privKey, pubKey, nil
}

func NewAESCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aesgcm, nil
}

func CipherPayload(aead cipher.AEAD, payload []byte) ([]byte, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	cipherPayload := aead.Seal(nonce, nonce, payload, []byte("aloh-hell-yeah"))
	return cipherPayload, nil
}

func DecipherPayload(aead cipher.AEAD, data []byte) ([]byte, error) {
	nonceSize := aead.NonceSize()
	nonce, cipherPayload := data[:nonceSize], data[nonceSize:]

	payload, err := aead.Open(nil, nonce, cipherPayload, []byte("aloh-hell-yeah"))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func NewMasterKey(privateKey *ecdh.PrivateKey, publicKeyByte []byte) ([]byte, error) {
	publicKey, err := ecdh.X25519().NewPublicKey(publicKeyByte)
	if err != nil {
		return nil, err
	}
	secret, err := privateKey.ECDH(publicKey)
	if err != nil {

		return nil, err
	}
	hkdfReader := hkdf.New(sha256.New, secret, nil, nil)

	key := make([]byte, 32)

	if _, err = io.ReadFull(hkdfReader, key); err != nil {
		return nil, err
	}
	return key, nil
}
