package frame

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

var ErrClosed = errors.New("frame: pipe closed")

type Pipe struct {
	in     <-chan []byte
	out    chan<- []byte
	done   chan struct{}
	peer   chan struct{}
	closer sync.Once
}

func NewPipe() (*Pipe, *Pipe) {
	left, right := make(chan []byte, 64), make(chan []byte, 64)
	leftDone, rightDone := make(chan struct{}), make(chan struct{})
	return &Pipe{in: left, out: right, done: leftDone, peer: rightDone},
		&Pipe{in: right, out: left, done: rightDone, peer: leftDone}
}

func (p *Pipe) ReadMessage(ctx context.Context) ([]byte, error) {
	select {
	case message := <-p.in:
		return message, nil
	case <-p.done:
		return nil, ErrClosed
	case <-p.peer:
		select {
		case message := <-p.in:
			return message, nil
		default:
			return nil, ErrClosed
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pipe) WriteMessage(ctx context.Context, message []byte) error {
	select {
	case <-p.done:
		return ErrClosed
	case <-p.peer:
		return ErrClosed
	default:
	}
	select {
	case p.out <- bytes.Clone(message):
		return nil
	case <-p.done:
		return ErrClosed
	case <-p.peer:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pipe) Close() error {
	p.closer.Do(func() { close(p.done) })
	return nil
}
