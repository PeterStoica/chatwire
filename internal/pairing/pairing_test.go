package pairing_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/testkit/fakephone"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestKnownAnswersFromAnIndependentImplementation(t *testing.T) {
	salt := make([]byte, 32)
	for i := range salt {
		salt[i] = byte(i)
	}
	key, err := pairing.LinkingKey("ABCD1234", salt)
	if err != nil || !bytes.Equal(key, mustHex(t, "b04d4d0e43ffa6d1a3b2226d4ee58ba2900a470c09bcf39fb5257512a55c08bc")) {
		t.Fatalf("LinkingKey = %x, %v", key, err)
	}
	secret, err := pairing.AdvSecretFrom(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), bytes.Repeat([]byte{3}, 32))
	if err != nil || !bytes.Equal(secret, mustHex(t, "8a82f12de8f65571e0a25b79095ae8a5dc09a1bd72e0a886e25d5c9818087c4d")) {
		t.Fatalf("AdvSecretFrom = %x, %v", secret, err)
	}
	bundle, err := pairing.BundleKey(bytes.Repeat([]byte{4}, 32), bytes.Repeat([]byte{5}, 32))
	if err != nil || !bytes.Equal(bundle, mustHex(t, "a676e91226b857e2e5ec2d0f92d832886a05404ea7e8165186ddd649112bcde7")) {
		t.Fatalf("BundleKey = %x, %v", bundle, err)
	}
	if got := pairing.EncodeLinkingCode([]byte{0, 1, 2, 3, 0xff}); got != "111H51ZZ" {
		t.Fatalf("EncodeLinkingCode(00010203ff) = %s", got)
	}
	if got := pairing.EncodeLinkingCode(bytes.Repeat([]byte{0xff}, 5)); got != "ZZZZZZZZ" {
		t.Fatalf("EncodeLinkingCode(ffffffffff) = %s", got)
	}
	if got := pairing.IdentityHMAC(bytes.Repeat([]byte{9}, 32), []byte("details"), false); !bytes.Equal(got, mustHex(t, "c3892dd54095530b9c72617a6d36648accf3622c70097f4576d6e958030b495a")) {
		t.Fatalf("IdentityHMAC = %x", got)
	}
	if got := pairing.IdentityHMAC(bytes.Repeat([]byte{9}, 32), []byte("details"), true); !bytes.Equal(got, mustHex(t, "41d829f4d89ccd239d4f62d3e194bc6790d797057b91f86094c8fb75124fb173")) {
		t.Fatalf("hosted IdentityHMAC = %x", got)
	}
	if pairing.CodeIterations != 131072 {
		t.Fatalf("CodeIterations = %d", pairing.CodeIterations)
	}
}

func companion(t *testing.T, seed byte) (pairing.Companion, *rand.ChaCha8) {
	t.Helper()
	random := rand.NewChaCha8([32]byte{seed})
	noise, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := pairing.NewAdvSecret(random)
	if err != nil {
		t.Fatal(err)
	}
	return pairing.Companion{Noise: noise, Identity: identity, AdvSecret: secret}, random
}

func phone(t *testing.T, random *rand.ChaCha8, hosted bool) *fakephone.Phone {
	t.Helper()
	account, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	return &fakephone.Phone{
		Account: account, Random: random, KeyIndex: 3, Hosted: hosted,
		JID: node.JID{User: "40700000000", Device: 9, Server: node.ServerUser},
		LID: node.JID{User: "123456789", Device: 9, Server: node.ServerLID},
	}
}

func TestQRCarriesTheCompanionKeys(t *testing.T) {
	c, _ := companion(t, 1)
	data := pairing.QRData([]byte("2@ref"), c, pairing.ClientOtherWeb)
	scan, err := fakephone.ParseQR(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(scan.Ref) != "2@ref" || scan.Noise != c.Noise.Public() || scan.Identity != c.Identity.Public() ||
		!bytes.Equal(scan.AdvSecret, c.AdvSecret) || scan.Client != "9" {
		t.Fatalf("scan = %+v", scan)
	}
	if !strings.HasPrefix(data, "https://wa.me/settings/linked_devices#2@ref,") {
		t.Fatalf("QR data = %s", data)
	}
}

func TestQRPairingEndToEnd(t *testing.T) {
	for _, hosted := range []bool{false, true} {
		c, random := companion(t, 2)
		p := phone(t, random, hosted)
		scan, err := fakephone.ParseQR(pairing.QRData([]byte("ref"), c, pairing.ClientOtherWeb))
		if err != nil {
			t.Fatal(err)
		}
		success, err := p.PairSuccess("42", scan.Identity, scan.AdvSecret)
		if err != nil {
			t.Fatal(err)
		}
		reply, account, err := pairing.HandlePairSuccess(success, c, random)
		if err != nil {
			t.Fatalf("hosted=%v: %v", hosted, err)
		}
		if err := p.VerifyDeviceSign(reply, c.Identity.Public()); err != nil {
			t.Fatalf("hosted=%v: phone rejects our device signature: %v", hosted, err)
		}
		if account.JID != p.JID || account.LID != p.LID || account.KeyIndex != 3 || account.Platform != "android" || account.AccountKey != p.Account.Public() {
			t.Fatalf("account = %+v", account)
		}
		if id, _ := reply.Attr("id").Text(); id != "42" {
			t.Fatalf("reply id = %q", id)
		}
	}
}

func TestPairSuccessRejections(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(success *node.Node, c *pairing.Companion)
		want   error
		code   string
	}{
		{name: "wrong adv secret", tamper: func(_ *node.Node, c *pairing.Companion) { c.AdvSecret = bytes.Repeat([]byte{1}, 32) }, want: pairing.ErrHMAC, code: "401"},
		{name: "flipped hmac", tamper: func(s *node.Node, _ *pairing.Companion) { flipInContainer(s, 3) }, want: pairing.ErrHMAC, code: "401"},
		{name: "signed for another companion", tamper: func(_ *node.Node, c *pairing.Companion) {
			other, err := curve.NewKeyPair(rand.NewChaCha8([32]byte{99}))
			if err != nil {
				panic(err)
			}
			c.Identity = other
		}, want: pairing.ErrAccountSignature, code: "401"},
		{name: "no pair-success", tamper: func(s *node.Node, _ *pairing.Companion) { s.Children = nil }, want: pairing.ErrMalformed, code: "401"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, random := companion(t, 3)
			p := phone(t, random, false)
			success, err := p.PairSuccess("7", c.Identity.Public(), c.AdvSecret)
			if err != nil {
				t.Fatal(err)
			}
			tt.tamper(&success, &c)
			reply, _, err := pairing.HandlePairSuccess(success, c, random)
			if !errors.Is(err, tt.want) {
				t.Fatalf("HandlePairSuccess error = %v, want %v", err, tt.want)
			}
			errorNode, ok := reply.Child("error")
			if kind, _ := reply.Attr("type").Text(); !ok || kind != "error" {
				t.Fatalf("reply = %s, want an iq error", reply)
			}
			if code, _ := errorNode.Attr("code").Text(); code != tt.code {
				t.Fatalf("error code = %s, want %s", code, tt.code)
			}
			if text, _ := errorNode.Attr("text").Text(); text != "not-authorized" {
				t.Fatalf("error text = %s, want not-authorized, as the official client replies", text)
			}
		})
	}
}

func flipInContainer(success *node.Node, offset int) {
	inner := &success.Children[0]
	for i := range inner.Children {
		if identity := &inner.Children[i]; identity.Tag == "device-identity" {
			identity.Bytes = bytes.Clone(identity.Bytes)
			identity.Bytes[len(identity.Bytes)-offset] ^= 1
		}
	}
}

func TestCodePairingEndToEnd(t *testing.T) {
	c, random := companion(t, 4)
	p := phone(t, random, false)
	request, hello, err := pairing.StartCode(random, "40700000000", c, pairing.ClientChrome, "Chrome (Linux)", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(request.Display()) != 9 || request.Display()[4] != '-' || request.JID.User != "40700000000" {
		t.Fatalf("code %q for %v", request.Display(), request.JID)
	}
	reg, _ := hello.Child("link_code_companion_reg")
	platform, _ := reg.Child("companion_platform_id")
	if reg.Attr("should_show_push_notification").String() != "true" || string(platform.Bytes) != "1" {
		t.Fatalf("the first code must push a notification to the phone and name the Chrome platform: %s", hello)
	}
	response, err := p.ReceiveHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.AcceptRef(response); err != nil {
		t.Fatal(err)
	}
	notification, err := p.TypeCode(request.Display())
	if err != nil {
		t.Fatal(err)
	}
	finish, clientSecret, err := request.Finish(notification, c, random)
	if err != nil {
		t.Fatal(err)
	}
	phoneSecret, companionIdentity, err := p.CompanionFinish(finish)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(clientSecret, phoneSecret) || companionIdentity != c.Identity.Public() {
		t.Fatal("client and phone derived different adv secrets")
	}
	c.AdvSecret = clientSecret
	success, err := p.PairSuccess("8", companionIdentity, phoneSecret)
	if err != nil {
		t.Fatal(err)
	}
	reply, _, err := pairing.HandlePairSuccess(success, c, random)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.VerifyDeviceSign(reply, c.Identity.Public()); err != nil {
		t.Fatal(err)
	}
}

func TestCodePairingFailsWhenPhoneTypesAnotherCode(t *testing.T) {
	c, random := companion(t, 5)
	p := phone(t, random, false)
	request, hello, err := pairing.StartCode(random, "40700000000", c, pairing.ClientChrome, "Chrome", false)
	if err != nil {
		t.Fatal(err)
	}
	wrong := []byte(request.Code)
	wrong[0] = map[bool]byte{true: '2', false: '1'}[wrong[0] == '1']
	response, err := p.ReceiveHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.AcceptRef(response); err != nil {
		t.Fatal(err)
	}
	notification, err := p.TypeCode(string(wrong))
	if err != nil {
		t.Fatal(err)
	}
	finish, _, err := request.Finish(notification, c, random)
	if err == nil {
		_, _, err = p.CompanionFinish(finish)
	}
	if err == nil {
		t.Fatal("pairing completed although the phone typed another code")
	}
}

func TestParsePhone(t *testing.T) {
	for _, phoneNumber := range []string{"", "12345", "683400", "0721234567", "+40 7a1 234 567", "+"} {
		if got, err := pairing.ParsePhone(phoneNumber); !errors.Is(err, pairing.ErrPhone) {
			t.Errorf("ParsePhone(%q) = %q, %v, want %v", phoneNumber, got, err, pairing.ErrPhone)
		}
	}
	for phoneNumber, want := range map[string]pairing.Phone{
		"+1 (555) 010-9999": "15550109999",
		"+683 4002":         "6834002",
		"+40 700-000 000":   "40700000000",
		"40700000000":       "40700000000",
	} {
		if got, err := pairing.ParsePhone(phoneNumber); err != nil || got != want {
			t.Errorf("ParsePhone(%q) = %q, %v, want %q", phoneNumber, got, err, want)
		}
	}
}

func TestAccountsOwnTheirNumberAndLID(t *testing.T) {
	t.Parallel()
	account := pairing.Account{JID: node.JID{User: "40711111111", Device: 3, Server: node.ServerUser}, LID: node.JID{User: "88111", Device: 3, Server: node.ServerLID}}
	tests := []struct {
		name string
		jid  node.JID
		owns bool
	}{
		{name: "the number", jid: node.JID{User: "40711111111", Server: node.ServerUser}, owns: true},
		{name: "another device of the number", jid: node.JID{User: "40711111111", Device: 9, Server: node.ServerUser}, owns: true},
		{name: "the lid", jid: node.JID{User: "88111", Server: node.ServerLID}, owns: true},
		{name: "a device of the lid", jid: node.JID{User: "88111", Device: 2, Server: node.ServerLID}, owns: true},
		{name: "someone else", jid: node.JID{User: "40722222222", Server: node.ServerUser}, owns: false},
		{name: "the number on the lid server", jid: node.JID{User: "40711111111", Server: node.ServerLID}, owns: false},
		{name: "the lid user on the phone server", jid: node.JID{User: "88111", Server: node.ServerUser}, owns: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := account.Owns(tt.jid); got != tt.owns {
				t.Fatalf("Owns(%v) = %v", tt.jid, got)
			}
		})
	}
	noLID := pairing.Account{JID: account.JID}
	if noLID.Owns(node.JID{Server: node.ServerLID}) {
		t.Fatal("an account without a lid owns the empty lid")
	}
}
