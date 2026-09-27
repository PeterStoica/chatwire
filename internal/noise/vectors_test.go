package noise_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/noise"
)

type vector struct {
	Source        string          `json:"source"`
	Prologue      hexBytes        `json:"init_prologue"`
	InitStatic    hexBytes        `json:"init_static"`
	InitEphemeral hexBytes        `json:"init_ephemeral"`
	RespStatic    hexBytes        `json:"resp_static"`
	RespEphemeral hexBytes        `json:"resp_ephemeral"`
	HandshakeHash hexBytes        `json:"handshake_hash"`
	Messages      []vectorMessage `json:"messages"`
}

type vectorMessage struct {
	Payload    hexBytes `json:"payload"`
	Ciphertext hexBytes `json:"ciphertext"`
}

type hexBytes []byte

func (h *hexBytes) UnmarshalText(text []byte) error {
	decoded, err := hex.AppendDecode(nil, text)
	*h = decoded
	return err
}

func TestXXMatchesPublishedVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/xx_25519_aesgcm_sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []vector
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || vectors[0].Source != "cacophony" {
		t.Fatalf("loaded %d vectors, want the published cacophony vector", len(vectors))
	}
	for _, v := range vectors {
		t.Run(v.Source, func(t *testing.T) {
			t.Parallel()
			runVector(t, v)
		})
	}
}

func runVector(t *testing.T, v vector) {
	t.Helper()
	initiator, err := noise.NewInitiator(noise.Standard, v.Prologue, keyPair(t, v.InitStatic), bytes.NewReader(v.InitEphemeral))
	if err != nil {
		t.Fatal(err)
	}
	responder, err := noise.NewResponder(noise.Standard, v.Prologue, keyPair(t, v.RespStatic), bytes.NewReader(v.RespEphemeral))
	if err != nil {
		t.Fatal(err)
	}

	hello := initiator.WriteHello(v.Messages[0].Payload)
	expect(t, 0, v.Messages[0], hello.Ephemeral[:], hello.Payload)
	responder.ReadHello(hello)

	reply, err := responder.WriteReply(v.Messages[1].Payload)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, 1, v.Messages[1], reply.Ephemeral[:], reply.Static, reply.Payload)
	if got, err := initiator.ReadReply(reply); err != nil || !bytes.Equal(got, v.Messages[1].Payload) {
		t.Fatalf("message 1 payload %x, %v", got, err)
	}

	finish, err := initiator.WriteFinish(v.Messages[2].Payload)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, 2, v.Messages[2], finish.Static, finish.Payload)
	if got, err := responder.ReadFinish(finish); err != nil || !bytes.Equal(got, v.Messages[2].Payload) {
		t.Fatalf("message 2 payload %x, %v", got, err)
	}

	if v.HandshakeHash != nil && !bytes.Equal(initiator.HandshakeHash(), v.HandshakeHash) {
		t.Fatalf("handshake hash %x, want %x", initiator.HandshakeHash(), v.HandshakeHash)
	}
	if initiator.PeerStatic() != keyPair(t, v.RespStatic).Public() || responder.PeerStatic() != keyPair(t, v.InitStatic).Public() {
		t.Fatal("peers did not learn each other's static keys")
	}
	checkTransport(t, v, initiator, responder)
}

func checkTransport(t *testing.T, v vector, initiator *noise.Initiator, responder *noise.Responder) {
	t.Helper()
	initiatorTransport, err := initiator.Split()
	if err != nil {
		t.Fatal(err)
	}
	responderTransport, err := responder.Split()
	if err != nil {
		t.Fatal(err)
	}
	for i, message := range v.Messages[3:] {
		sender, receiver := initiatorTransport, responderTransport
		if i%2 == 0 {
			sender, receiver = responderTransport, initiatorTransport
		}
		ciphertext := sender.Seal(message.Payload)
		expect(t, i+3, message, ciphertext)
		plaintext, err := receiver.OpenInPlace(ciphertext)
		if err != nil || !bytes.Equal(plaintext, message.Payload) {
			t.Fatalf("transport message %d decrypted to %x, %v", i+3, plaintext, err)
		}
	}
}

func expect(t *testing.T, index int, want vectorMessage, parts ...[]byte) {
	t.Helper()
	if got := bytes.Join(parts, nil); !bytes.Equal(got, want.Ciphertext) {
		t.Fatalf("message %d ciphertext\n got %x\nwant %x", index, got, want.Ciphertext)
	}
}

func keyPair(t *testing.T, private []byte) curve.KeyPair {
	t.Helper()
	kp, err := curve.NewKeyPair(bytes.NewReader(private))
	if err != nil {
		t.Fatal(err)
	}
	return kp
}
