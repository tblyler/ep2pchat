package signer

// Signer implements encoding and decoding methods for messages
type Signer interface {
	Encode(rawMsg []byte) ([]byte, error)
	Decode(encMsg []byte) ([]byte, error)
}
