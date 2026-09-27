package signal_test

import (
	"encoding"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/signal"
)

func TestUnreadableRecordsAreRefused(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target encoding.BinaryUnmarshaler
	}{
		{"session", &signal.Session{}},
		{"sender key", &signal.SenderKey{}},
		{"sender keys", &signal.SenderKeys{}},
	} {
		for _, raw := range []string{`{`, `{"version":2}`, `{"version":"one"}`, `[]`} {
			if err := tt.target.UnmarshalBinary([]byte(raw)); !errors.Is(err, signal.ErrRecord) {
				t.Errorf("%s from %s: %v, want %v", tt.name, raw, err, signal.ErrRecord)
			}
		}
	}
}

func TestAnEmptySessionStaysEmpty(t *testing.T) {
	var missing *signal.Session
	raw, err := missing.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var restored signal.Session
	if err := restored.UnmarshalBinary(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Encrypt([]byte("x")); !errors.Is(err, signal.ErrNoSession) {
		t.Fatalf("Encrypt on a restored empty session: %v", err)
	}
}
