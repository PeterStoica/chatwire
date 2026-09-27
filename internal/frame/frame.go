package frame

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

const (
	lengthSize = 3
	MaxSize    = 1<<24 - 1
)

var (
	ErrTooLarge  = errors.New("frame: payload too large")
	ErrBadHeader = errors.New("frame: unexpected connection header")
)

type MessageConn interface {
	ReadMessage(ctx context.Context) ([]byte, error)
	WriteMessage(ctx context.Context, message []byte) error
	Close() error
}

type Conn struct {
	messages MessageConn
	header   []byte
	expect   []byte
	pending  []byte
}

func Client(messages MessageConn, header []byte) *Conn {
	return &Conn{messages: messages, header: header}
}

func Server(messages MessageConn, header []byte) *Conn {
	return &Conn{messages: messages, expect: header}
}

func (c *Conn) Write(ctx context.Context, payload []byte) error {
	size := len(payload)
	if size > MaxSize {
		return fmt.Errorf("%w: %d bytes", ErrTooLarge, size)
	}
	message := make([]byte, 0, len(c.header)+lengthSize+size)
	message = append(message, c.header...)
	message = append(message, byte(size>>16), byte(size>>8), byte(size))
	message = append(message, payload...)
	if err := c.messages.WriteMessage(ctx, message); err != nil {
		return fmt.Errorf("frame: write: %w", err)
	}
	c.header = nil
	return nil
}

func (c *Conn) Read(ctx context.Context) ([]byte, error) {
	for {
		if payload, ok, err := c.next(); err != nil || ok {
			return payload, err
		}
		message, err := c.messages.ReadMessage(ctx)
		if err != nil {
			return nil, fmt.Errorf("frame: read: %w", err)
		}
		if len(c.pending) == 0 {
			c.pending = message
			continue
		}
		c.pending = append(c.pending, message...)
	}
}

func (c *Conn) Close() error {
	if err := c.messages.Close(); err != nil {
		return fmt.Errorf("frame: close: %w", err)
	}
	return nil
}

func (c *Conn) next() ([]byte, bool, error) {
	if c.expect != nil {
		seen := min(len(c.pending), len(c.expect))
		if !bytes.Equal(c.pending[:seen], c.expect[:seen]) {
			return nil, false, fmt.Errorf("%w: %x", ErrBadHeader, c.pending[:seen])
		}
		if seen < len(c.expect) {
			return nil, false, nil
		}
		c.pending, c.expect = c.pending[seen:], nil
	}
	if len(c.pending) < lengthSize {
		return nil, false, nil
	}
	size := int(c.pending[0])<<16 | int(c.pending[1])<<8 | int(c.pending[2])
	if len(c.pending) < lengthSize+size {
		return nil, false, nil
	}
	payload := c.pending[lengthSize : lengthSize+size : lengthSize+size]
	c.pending = c.pending[lengthSize+size:]
	if len(c.pending) == 0 {
		c.pending = nil
	}
	return payload, true, nil
}
