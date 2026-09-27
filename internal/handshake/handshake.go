package handshake

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/PeterStoica/chatwire/internal/cert"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/noise"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const protocolMagic = 6

func Header(dictVersion byte) []byte {
	return []byte{'W', 'A', protocolMagic, dictVersion}
}

type Config struct {
	DictVersion byte
	Root        curve.PublicKey
	Static      curve.KeyPair
	Payload     []byte
	Random      io.Reader
	Now         func() time.Time
}

type Conn struct {
	frames    *frame.Conn
	transport *noise.Transport
}

func NewConn(frames *frame.Conn, transport *noise.Transport) *Conn {
	return &Conn{frames: frames, transport: transport}
}

func (c *Conn) Read(ctx context.Context) ([]byte, error) {
	ciphertext, err := c.frames.Read(ctx)
	if err != nil {
		return nil, err
	}
	return c.transport.OpenInPlace(ciphertext)
}

func (c *Conn) Write(ctx context.Context, plaintext []byte) error {
	return c.frames.Write(ctx, c.transport.Seal(plaintext))
}

func (c *Conn) Close() error {
	return c.frames.Close()
}

func Initiate(ctx context.Context, messages frame.MessageConn, cfg Config) (*Conn, error) {
	header := Header(cfg.DictVersion)
	frames := frame.Client(messages, header)
	initiator, err := noise.NewInitiator(noise.WhatsApp, header, cfg.Static, cfg.Random)
	if err != nil {
		return nil, err
	}
	hello := initiator.WriteHello(nil)
	if err := frames.Write(ctx, signon.EncodeClientHello(hello.Ephemeral[:])); err != nil {
		return nil, err
	}

	raw, err := frames.Read(ctx)
	if err != nil {
		return nil, err
	}
	reply, err := parseReply(raw)
	if err != nil {
		return nil, err
	}
	chain, err := initiator.ReadReply(reply)
	if err != nil {
		return nil, err
	}
	if err := cert.Verify(chain, initiator.PeerStatic(), cfg.Root, cfg.Now()); err != nil {
		return nil, err
	}

	finish, err := initiator.WriteFinish(cfg.Payload)
	if err != nil {
		return nil, err
	}
	if err := frames.Write(ctx, signon.EncodeClientFinish(signon.ClientFinish{Static: finish.Static, Payload: finish.Payload})); err != nil {
		return nil, err
	}
	transport, err := initiator.Split()
	if err != nil {
		return nil, err
	}
	return NewConn(frames, transport), nil
}

func parseReply(raw []byte) (noise.Reply, error) {
	hello, err := signon.ParseServerHello(raw)
	if err != nil {
		return noise.Reply{}, err
	}
	ephemeral, err := curve.ParsePublicKey(hello.Ephemeral)
	if err != nil {
		return noise.Reply{}, fmt.Errorf("handshake: server ephemeral: %w", err)
	}
	return noise.Reply{Ephemeral: ephemeral, Static: hello.Static, Payload: hello.Payload}, nil
}
