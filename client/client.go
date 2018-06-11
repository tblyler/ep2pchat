package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

type ClientID uint64

var currentClientID uint64

func getID() ClientID {
	return ClientID(atomic.AddUint64(&currentClientID, 1))
}

type Client struct {
	conn      net.Conn
	id        ClientID
	ctx       context.Context
	cancel    context.CancelFunc
	writeLock sync.Mutex
	readLock  sync.Mutex
}

func NewClient(conn net.Conn, ctx context.Context) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		conn:   conn,
		id:     getID(),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Client) GetID() ClientID {
	return c.id
}

func (c *Client) SendMsg(msg []byte) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	sizeBytes := make([]byte, 8)
	binary.PutVarint(sizeBytes, int64(len(msg)))

	_, err := io.Copy(c.conn, bytes.NewReader(append(sizeBytes, msg...)))
	return err
}

func (c *Client) GetMsg() ([]byte, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()

	sizeBytes := make([]byte, 8)
	_, err := io.ReadFull(c.conn, sizeBytes)
	if err != nil {
		return nil, err
	}

	msgSize, errInt := binary.Varint(sizeBytes)
	if errInt <= 0 || msgSize <= 0 {
		return nil, errors.New("Failed to decode message size bytes")
	}

	output := bytes.NewBuffer(make([]byte, msgSize))
	output.Reset()

	_, err = io.CopyN(output, c.conn, msgSize)
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

func (c *Client) Done() bool {
	return c.ctx.Err() != nil
}

func (c *Client) Close() error {
	c.cancel()
	return c.conn.Close()
}
