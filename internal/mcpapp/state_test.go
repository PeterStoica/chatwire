package mcpapp

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

func TestLinkedStateSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "linked.json")
	if _, linked, err := load(path); err != nil || linked {
		t.Fatalf("load() before linking = %v, %v", linked, err)
	}
	want := linkflow.Linked{
		Identity:  device.Stored{Noise: [curve.KeySize]byte{1}, Identity: [curve.KeySize]byte{2}, RegistrationID: 7},
		AdvSecret: []byte{3, 4},
		Account: pairing.Account{
			JID:      node.JID{User: "40700000000", Device: 17, Server: node.ServerUser},
			LID:      node.JID{User: "987654321", Device: 17, Server: node.ServerLID},
			Platform: "android", KeyIndex: 2, AccountKey: curve.PublicKey{9}, SignedIdentity: []byte{5},
		},
	}
	state := client.State{Linked: want, PreKeys: map[uint32][32]byte{1: {7}, 16777215: {9}}, NextPreKeyID: 2}
	if err := save(path, state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode %v, want owner-only", info.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file left behind: %v", err)
	}
	loaded, linked, err := load(path)
	if err != nil || !linked || !reflect.DeepEqual(loaded, state) {
		t.Fatalf("load() = %+v, %v, %v", loaded, linked, err)
	}
}

func TestCorruptStateIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "linked.json")
	if err := os.WriteFile(path, []byte(`{"Account":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, linked, err := load(path); err == nil || linked {
		t.Fatalf("load() of a truncated file = %v, %v", linked, err)
	}
}
