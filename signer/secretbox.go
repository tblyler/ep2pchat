package signer

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	NonceLength = 24
	KeyLength   = 32
)

type Secretbox struct {
	key [KeyLength]byte
}

func NewSecretBox(key [KeyLength]byte) *Secretbox {
	return &Secretbox{
		key: key,
	}
}

func (s *Secretbox) Encode(rawMsg []byte) (encMsg []byte, err error) {
	nonce, err := generateNonce()
	if err != nil {
		return
	}

	encMsg = secretbox.Seal(nonce[:], rawMsg, &nonce, &s.key)
	return
}

func (s *Secretbox) Decode(encMsg []byte) (msg []byte, err error) {
	if len(encMsg) < NonceLength {
		err = errors.New("Invalid message length for decode")
		return
	}

	var nonce [NonceLength]byte
	copy(nonce[:], encMsg[:NonceLength])

	msg, ok := secretbox.Open(nil, encMsg[NonceLength:], &nonce, &s.key)
	if !ok {
		err = errors.New("Failed to decode message")
	}

	return
}

func generateNonce() (nonce [NonceLength]byte, err error) {
	_, err = io.ReadFull(rand.Reader, nonce[:])
	return nonce, err
}
