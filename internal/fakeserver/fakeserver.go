package fakeserver

import (
	"context"
	"io"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/handshake"
	"github.com/PeterStoica/chatwire/internal/noise"
	"github.com/PeterStoica/chatwire/internal/signon"
)

type Config struct {
	DictVersion byte
	Static      curve.KeyPair
	Chain       []byte
	Random      io.Reader
}

type Session struct {
	Conn         *handshake.Conn
	ClientStatic curve.PublicKey
	Payload      []byte
}

func Respond(ctx context.Context, messages frame.MessageConn, cfg Config) (Session, error) {
	header := handshake.Header(cfg.DictVersion)
	frames := frame.Server(messages, header)
	raw, err := frames.Read(ctx)
	if err != nil {
		return Session{}, err
	}
	ephemeralRaw, err := signon.ParseClientHello(raw)
	if err != nil {
		return Session{}, err
	}
	clientEphemeral, err := curve.ParsePublicKey(ephemeralRaw)
	if err != nil {
		return Session{}, err
	}
	responder, err := noise.NewResponder(noise.WhatsApp, header, cfg.Static, cfg.Random)
	if err != nil {
		return Session{}, err
	}
	responder.ReadHello(noise.Hello{Ephemeral: clientEphemeral})
	reply, err := responder.WriteReply(cfg.Chain)
	if err != nil {
		return Session{}, err
	}
	hello := signon.EncodeServerHello(signon.ServerHello{Ephemeral: reply.Ephemeral[:], Static: reply.Static, Payload: reply.Payload})
	if err := frames.Write(ctx, hello); err != nil {
		return Session{}, err
	}

	raw, err = frames.Read(ctx)
	if err != nil {
		return Session{}, err
	}
	finish, err := signon.ParseClientFinish(raw)
	if err != nil {
		return Session{}, err
	}
	payload, err := responder.ReadFinish(noise.Finish{Static: finish.Static, Payload: finish.Payload})
	if err != nil {
		return Session{}, err
	}
	transport, err := responder.Split()
	if err != nil {
		return Session{}, err
	}
	return Session{Conn: handshake.NewConn(frames, transport), ClientStatic: responder.PeerStatic(), Payload: payload}, nil
}
