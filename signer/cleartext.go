package signer

type ClearText struct {
}

func NewClearText() *ClearText {
	return new(ClearText)
}

func (c *ClearText) Encode(rawMsg []byte) ([]byte, error) {
	return rawMsg, nil
}

func (c *ClearText) Decode(encMsg []byte) ([]byte, error) {
	return encMsg, nil
}
