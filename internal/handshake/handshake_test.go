package handshake_test

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"

	"github.com/PeterStoica/chatwire/internal/cert"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/handshake"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeserver"
)

const dictVersion = 3

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type world struct {
	random    *rand.ChaCha8
	authority *cert.Authority
	server    curve.KeyPair
	client    curve.KeyPair
}

func newWorld(t *testing.T, seed byte) *world {
	t.Helper()
	random := rand.NewChaCha8([32]byte{seed})
	authority, err := cert.NewAuthority(random)
	if err != nil {
		t.Fatal(err)
	}
	return &world{random: random, authority: authority, server: keyPair(t, random), client: keyPair(t, random)}
}

func keyPair(t *testing.T, random *rand.ChaCha8) curve.KeyPair {
	t.Helper()
	kp, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	return kp
}

func (w *world) chain(t *testing.T, leaf curve.PublicKey, validity cert.Validity) []byte {
	t.Helper()
	chain, err := w.authority.Issue(w.random, leaf, validity)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

func around(now time.Time) cert.Validity {
	return cert.Validity{NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
}

type serverResult struct {
	session fakeserver.Session
	err     error
}

func (w *world) serve(ctx context.Context, messages frame.MessageConn, chain []byte) <-chan serverResult {
	done := make(chan serverResult, 1)
	go func() {
		session, err := fakeserver.Respond(ctx, messages, fakeserver.Config{DictVersion: dictVersion, Static: w.server, Chain: chain, Random: w.random})
		done <- serverResult{session: session, err: err}
	}()
	return done
}

func (w *world) initiate(ctx context.Context, messages frame.MessageConn, payload []byte) (*handshake.Conn, error) {
	return handshake.Initiate(ctx, messages, handshake.Config{
		DictVersion: dictVersion, Root: w.authority.Root(), Static: w.client, Payload: payload, Random: w.random, Now: time.Now,
	})
}

func TestInitiateAgreesWithResponder(t *testing.T) {
	for seed := range byte(200) {
		synctest.Test(t, func(t *testing.T) {
			w := newWorld(t, seed)
			clientSide, serverSide := frame.NewPipe()
			server := w.serve(t.Context(), serverSide, w.chain(t, w.server.Public(), around(time.Now())))
			payload := []byte{seed, 1, 2, 3}
			client, err := w.initiate(t.Context(), clientSide, payload)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			result := <-server
			if result.err != nil {
				t.Fatalf("seed %d: server: %v", seed, result.err)
			}
			if !bytes.Equal(result.session.Payload, payload) || result.session.ClientStatic != w.client.Public() {
				t.Fatalf("seed %d: server saw payload %x from %x", seed, result.session.Payload, result.session.ClientStatic)
			}
			assertDuplex(t, client, result.session.Conn)
		})
	}
}

func assertDuplex(t *testing.T, client, server *handshake.Conn) {
	t.Helper()
	for i := range byte(3) {
		exchange(t, client, server, []byte{0xc0, i})
		exchange(t, server, client, []byte{0x5e, i})
	}
}

func exchange(t *testing.T, from, to *handshake.Conn, message []byte) {
	t.Helper()
	if err := from.Write(t.Context(), message); err != nil {
		t.Fatal(err)
	}
	got, err := to.Read(t.Context())
	if err != nil || !bytes.Equal(got, message) {
		t.Fatalf("sent %x, received %x, %v", message, got, err)
	}
}

func TestInitiateRejects(t *testing.T) {
	tests := []struct {
		name  string
		wait  time.Duration
		chain func(t *testing.T, w *world, now time.Time) []byte
	}{
		{
			name: "chain signed by another root",
			chain: func(t *testing.T, w *world, now time.Time) []byte {
				t.Helper()
				other, err := cert.NewAuthority(w.random)
				if err != nil {
					t.Fatal(err)
				}
				chain, err := other.Issue(w.random, w.server.Public(), around(now))
				if err != nil {
					t.Fatal(err)
				}
				return chain
			},
		},
		{
			name: "leaf issued for another key",
			chain: func(t *testing.T, w *world, now time.Time) []byte {
				t.Helper()
				return w.chain(t, keyPair(t, w.random).Public(), around(now))
			},
		},
		{
			name: "chain not yet valid",
			chain: func(t *testing.T, w *world, now time.Time) []byte {
				t.Helper()
				return w.chain(t, w.server.Public(), cert.Validity{NotBefore: now.Add(time.Second), NotAfter: now.Add(time.Hour)})
			},
		},
		{
			name: "chain expired a second ago",
			chain: func(t *testing.T, w *world, now time.Time) []byte {
				t.Helper()
				return w.chain(t, w.server.Public(), cert.Validity{NotBefore: now.Add(-time.Hour), NotAfter: now.Add(-time.Second)})
			},
		},
		{
			name: "chain expires while waiting",
			wait: 2 * time.Hour,
			chain: func(t *testing.T, w *world, now time.Time) []byte {
				t.Helper()
				return w.chain(t, w.server.Public(), around(now))
			},
		},
		{
			name: "garbage chain",
			chain: func(*testing.T, *world, time.Time) []byte {
				return []byte{0xff, 0xff, 0xff}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := newWorld(t, 1)
				chain := tt.chain(t, w, time.Now())
				time.Sleep(tt.wait)
				clientSide, serverSide := frame.NewPipe()
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				server := w.serve(ctx, serverSide, chain)
				_, err := w.initiate(t.Context(), clientSide, []byte("x"))
				if !errors.Is(err, cert.ErrRejected) {
					t.Fatalf("Initiate() error = %v, want %v", err, cert.ErrRejected)
				}
				cancel()
				<-server
			})
		})
	}
}

func TestInitiateAcceptsChainAtExactBoundaries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWorld(t, 2)
		now := time.Now()
		clientSide, serverSide := frame.NewPipe()
		server := w.serve(t.Context(), serverSide, w.chain(t, w.server.Public(), cert.Validity{NotBefore: now, NotAfter: now}))
		if _, err := w.initiate(t.Context(), clientSide, nil); err != nil {
			t.Fatalf("Initiate() = %v, want nil at the validity boundary", err)
		}
		if result := <-server; result.err != nil {
			t.Fatal(result.err)
		}
	})
}

func TestInitiateTimesOutExactlyWhenServerIsSilent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWorld(t, 4)
		clientSide, serverSide := frame.NewPipe()
		defer serverSide.Close()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		start := time.Now()
		_, err := w.initiate(ctx, clientSide, nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Initiate() error = %v, want %v", err, context.DeadlineExceeded)
		}
		if waited := time.Since(start); waited != 20*time.Second {
			t.Fatalf("gave up after %s, want exactly 20s", waited)
		}
	})
}

func TestInitiateFailsWhenServerHangsUpMidHandshake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWorld(t, 5)
		clientSide, serverSide := frame.NewPipe()
		go func() {
			if _, err := frame.Server(serverSide, handshake.Header(dictVersion)).Read(t.Context()); err != nil {
				t.Error(err)
			}
			serverSide.Close()
		}()
		_, err := w.initiate(t.Context(), clientSide, nil)
		if !errors.Is(err, frame.ErrClosed) {
			t.Fatalf("Initiate() error = %v, want %v", err, frame.ErrClosed)
		}
	})
}

func TestHeaderCarriesDictionaryVersion(t *testing.T) {
	if got := handshake.Header(7); !bytes.Equal(got, []byte{'W', 'A', 6, 7}) {
		t.Fatalf("Header(7) = %x", got)
	}
}
