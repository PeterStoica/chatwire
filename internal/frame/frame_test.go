package frame_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/PeterStoica/chatwire/internal/frame"
)

var header = []byte{'W', 'A', 6, 3}

type scripted struct {
	messages [][]byte
	written  [][]byte
}

func (s *scripted) ReadMessage(context.Context) ([]byte, error) {
	if len(s.messages) == 0 {
		return nil, io.EOF
	}
	next := s.messages[0]
	s.messages = s.messages[1:]
	return next, nil
}

func (s *scripted) WriteMessage(_ context.Context, message []byte) error {
	s.written = append(s.written, message)
	return nil
}

func (s *scripted) Close() error { return nil }

func TestServerReassemblesFramesUnderArbitraryChunking(t *testing.T) {
	for seed := range uint64(2000) {
		rng := rand.New(rand.NewPCG(seed, 11))
		payloads := make([][]byte, 1+rng.IntN(6))
		writer := &scripted{}
		client := frame.Client(writer, header)
		for i := range payloads {
			payloads[i] = randomPayload(rng)
			if err := client.Write(t.Context(), payloads[i]); err != nil {
				t.Fatal(err)
			}
		}
		server := frame.Server(&scripted{messages: rechunk(rng, bytes.Join(writer.written, nil))}, header)
		for i, want := range payloads {
			got, err := server.Read(t.Context())
			if err != nil {
				t.Fatalf("seed %d frame %d: %v", seed, i, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("seed %d frame %d: got %d bytes, want %d", seed, i, len(got), len(want))
			}
		}
		if _, err := server.Read(t.Context()); !errors.Is(err, io.EOF) {
			t.Fatalf("seed %d: read past the last frame = %v, want EOF", seed, err)
		}
	}
}

func TestClientSendsHeaderOnlyOnce(t *testing.T) {
	writer := &scripted{}
	client := frame.Client(writer, header)
	for _, payload := range [][]byte{{1}, {2, 3}} {
		if err := client.Write(t.Context(), payload); err != nil {
			t.Fatal(err)
		}
	}
	want := [][]byte{{'W', 'A', 6, 3, 0, 0, 1, 1}, {0, 0, 2, 2, 3}}
	if len(writer.written) != 2 || !bytes.Equal(writer.written[0], want[0]) || !bytes.Equal(writer.written[1], want[1]) {
		t.Fatalf("written = %x, want %x", writer.written, want)
	}
}

func TestServerRejectsWrongHeader(t *testing.T) {
	server := frame.Server(&scripted{messages: [][]byte{{'W', 'A', 6, 2, 0, 0, 0}}}, header)
	if _, err := server.Read(t.Context()); !errors.Is(err, frame.ErrBadHeader) {
		t.Fatalf("Read() error = %v, want %v", err, frame.ErrBadHeader)
	}
}

func TestWriteRejectsOversizedPayload(t *testing.T) {
	client := frame.Client(&scripted{}, nil)
	if err := client.Write(t.Context(), make([]byte, frame.MaxSize+1)); !errors.Is(err, frame.ErrTooLarge) {
		t.Fatalf("Write() error = %v, want %v", err, frame.ErrTooLarge)
	}
	if err := client.Write(t.Context(), make([]byte, frame.MaxSize)); err != nil {
		t.Fatalf("Write() of exactly MaxSize = %v, want nil", err)
	}
}

func TestPipeDeliversUntilClosed(t *testing.T) {
	left, right := frame.NewPipe()
	if err := left.WriteMessage(t.Context(), []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := left.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := right.ReadMessage(t.Context())
	if err != nil || string(got) != "hello" {
		t.Fatalf("ReadMessage() = %q, %v; want hello, nil", got, err)
	}
	if _, err := right.ReadMessage(t.Context()); !errors.Is(err, frame.ErrClosed) {
		t.Fatalf("ReadMessage() after peer close = %v, want %v", err, frame.ErrClosed)
	}
	if err := right.WriteMessage(t.Context(), []byte("late")); !errors.Is(err, frame.ErrClosed) {
		t.Fatalf("WriteMessage() to closed peer = %v, want %v", err, frame.ErrClosed)
	}
}

func rechunk(rng *rand.Rand, stream []byte) [][]byte {
	var chunks [][]byte
	for len(stream) > 0 {
		size := 1 + rng.IntN(min(len(stream), 1+rng.IntN(300)))
		chunks = append(chunks, stream[:size])
		stream = stream[size:]
	}
	return chunks
}

func randomPayload(rng *rand.Rand) []byte {
	sizes := []int{0, 1, 2, 3, 255, 256, 65535, 65536, 1 + rng.IntN(5000)}
	out := make([]byte, sizes[rng.IntN(len(sizes))])
	for i := range out {
		out[i] = byte(rng.Uint32())
	}
	return out
}

func TestServerAcceptsHeaderSplitAcrossMessages(t *testing.T) {
	server := frame.Server(&scripted{messages: [][]byte{{'W'}, {'A'}, {6, 3, 0}, {0, 1}, {9}}}, header)
	got, err := server.Read(t.Context())
	if err != nil || !bytes.Equal(got, []byte{9}) {
		t.Fatalf("Read() = %x, %v", got, err)
	}
}

func TestServerReturnsFrameArrivingWithTheHeader(t *testing.T) {
	server := frame.Server(&scripted{messages: [][]byte{{'W', 'A', 6, 3, 0, 0, 1, 9}}}, header)
	got, err := server.Read(t.Context())
	if err != nil || !bytes.Equal(got, []byte{9}) {
		t.Fatalf("Read() = %x, %v", got, err)
	}
}

func TestServerRejectsWrongPartialHeaderImmediately(t *testing.T) {
	server := frame.Server(&scripted{messages: [][]byte{{'W', 'B'}}}, header)
	if _, err := server.Read(t.Context()); !errors.Is(err, frame.ErrBadHeader) {
		t.Fatalf("Read() error = %v, want %v", err, frame.ErrBadHeader)
	}
}

func TestClientReturnsEOFOnPartialFrame(t *testing.T) {
	client := frame.Client(&scripted{messages: [][]byte{{0, 0, 5, 1, 2}}}, nil)
	if _, err := client.Read(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v, want %v", err, io.EOF)
	}
}

func TestPipeHonoursContext(t *testing.T) {
	left, _ := frame.NewPipe()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := left.ReadMessage(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadMessage() = %v, want %v", err, context.Canceled)
	}
	for range 64 {
		if err := left.WriteMessage(t.Context(), []byte{1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := left.WriteMessage(ctx, []byte{1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteMessage() on a full pipe = %v, want %v", err, context.Canceled)
	}
	if err := left.Close(); err != nil {
		t.Fatal(err)
	}
	if err := left.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
	if err := left.WriteMessage(t.Context(), []byte{1}); !errors.Is(err, frame.ErrClosed) {
		t.Fatalf("WriteMessage() after Close = %v, want %v", err, frame.ErrClosed)
	}
	if _, err := left.ReadMessage(t.Context()); !errors.Is(err, frame.ErrClosed) {
		t.Fatalf("ReadMessage() after Close = %v, want %v", err, frame.ErrClosed)
	}
}

func FuzzServerReassemblesAnyChunking(f *testing.F) {
	f.Add([]byte("hello, whatsapp"), []byte{3, 0, 200}, []byte{1, 2, 3})
	f.Add(bytes.Repeat([]byte{7}, 600), []byte{255, 255, 90}, []byte{4, 0, 17, 1})
	f.Fuzz(func(t *testing.T, stream, sizes, cuts []byte) {
		sizes = sizes[:min(len(sizes), 16)]
		payloads := make([][]byte, 0, len(sizes))
		for _, size := range sizes {
			n := min(int(size), len(stream))
			payloads, stream = append(payloads, stream[:n]), stream[n:]
		}
		writer := &scripted{}
		client := frame.Client(writer, header)
		for _, payload := range payloads {
			if err := client.Write(t.Context(), payload); err != nil {
				t.Fatal(err)
			}
		}
		wire := bytes.Join(writer.written, nil)
		var messages [][]byte
		for i := 0; len(wire) > 0; i++ {
			n := 1
			if len(cuts) > 0 {
				n = 1 + int(cuts[i%len(cuts)])
			}
			n = min(n, len(wire))
			messages, wire = append(messages, bytes.Clone(wire[:n])), wire[n:]
		}
		server := frame.Server(&scripted{messages: messages}, header)
		for i, want := range payloads {
			got, err := server.Read(t.Context())
			if err != nil {
				t.Fatalf("frame %d: %v", i, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("frame %d = %x, want %x", i, got, want)
			}
			_ = append(got, 0xee, 0xee, 0xee)
			for j := range got {
				got[j] ^= 0xff
			}
		}
		if _, err := server.Read(t.Context()); !errors.Is(err, io.EOF) {
			t.Fatalf("read past the last frame = %v, want EOF", err)
		}
	})
}
