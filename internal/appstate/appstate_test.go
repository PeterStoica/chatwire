package appstate_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeappstate"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func contact(full, first string) *wire.SyncActionValue {
	return &wire.SyncActionValue{ContactAction: &wire.SyncActionValue_ContactAction{FullName: new(full), FirstName: new(first)}}
}

func server(t *testing.T) *fakeappstate.Server {
	t.Helper()
	s, err := fakeappstate.New([]byte{0, 0, 0, 7}, bytes.Repeat([]byte{9}, 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestExpandingKeys(t *testing.T) {
	t.Parallel()
	if _, err := appstate.Expand(nil); !errors.Is(err, appstate.ErrKey) {
		t.Fatalf("Expand(nil) = %v", err)
	}
	a, err := appstate.Expand([]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	b, err := appstate.Expand([]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	parts := [][32]byte{a.Index, a.ValueEncryption, a.ValueMAC, a.SnapshotMAC, a.PatchMAC}
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			if parts[i] == parts[j] {
				t.Fatalf("parts %d and %d are equal", i, j)
			}
		}
	}
	if a == b {
		t.Fatal("different key data gave the same keys")
	}
}

func TestMutationsRoundTripAndRefuseTampering(t *testing.T) {
	t.Parallel()
	keys, err := appstate.Expand(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	keyID := []byte{1, 2}
	index := []string{"contact", "40744444444@s.whatsapp.net"}
	record, err := appstate.Encrypt(rand.Reader, wire.SyncdMutation_SET, index, contact("Mama Ioana", "Mama"), 5, keyID, keys)
	if err != nil {
		t.Fatal(err)
	}
	m, err := appstate.Decrypt(wire.SyncdMutation_SET, record, keys)
	if err != nil || m.Operation != wire.SyncdMutation_SET || len(m.Index) != 2 || m.Index[1] != index[1] || m.Version != 5 ||
		m.Value.GetContactAction().GetFullName() != "Mama Ioana" || !bytes.Equal(m.IndexMAC, record.GetIndex().GetBlob()) || len(m.ValueMAC) != 32 {
		t.Fatalf("Decrypt() = %+v, %v", m, err)
	}
	smallest, err := appstate.Encrypt(rand.Reader, wire.SyncdMutation_SET, []string{}, nil, 1, keyID, keys)
	if err != nil {
		t.Fatal(err)
	}
	if size := len(smallest.GetValue().GetBlob()); size != 64 {
		t.Fatalf("the smallest record is %d bytes, want one block of ciphertext", size)
	}
	if m, err := appstate.Decrypt(wire.SyncdMutation_SET, smallest, keys); err != nil || len(m.Index) != 0 {
		t.Fatalf("the smallest record: %+v, %v", m, err)
	}
	other, err := appstate.Expand(bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	tamper := func(change func(r *wire.SyncdRecord)) *wire.SyncdRecord {
		c := proto.CloneOf(record)
		change(c)
		return c
	}
	flip := func(at int) func(r *wire.SyncdRecord) {
		return func(r *wire.SyncdRecord) { r.Value.Blob[at] ^= 1 }
	}
	tests := []struct {
		name   string
		op     wire.SyncdMutation_SyncdOperation
		record *wire.SyncdRecord
		keys   appstate.Keys
		want   error
	}{
		{name: "wrong operation", op: wire.SyncdMutation_REMOVE, record: record, keys: keys, want: appstate.ErrValueMAC},
		{name: "unknown operation", op: 9, record: record, keys: keys, want: appstate.ErrMalformed},
		{name: "wrong key", op: wire.SyncdMutation_SET, record: record, keys: other, want: appstate.ErrValueMAC},
		{name: "flipped iv", op: wire.SyncdMutation_SET, record: tamper(flip(0)), keys: keys, want: appstate.ErrValueMAC},
		{name: "flipped mac", op: wire.SyncdMutation_SET, record: tamper(flip(len(record.GetValue().GetBlob()) - 1)), keys: keys, want: appstate.ErrValueMAC},
		{name: "other key id", op: wire.SyncdMutation_SET, record: tamper(func(r *wire.SyncdRecord) { r.KeyId.Id = []byte{9} }), keys: keys, want: appstate.ErrValueMAC},
		{name: "index mac of another record", op: wire.SyncdMutation_SET, record: tamper(func(r *wire.SyncdRecord) { r.Index.Blob[0] ^= 1 }), keys: keys, want: appstate.ErrIndexMAC},
		{name: "too short", op: wire.SyncdMutation_SET, record: tamper(func(r *wire.SyncdRecord) { r.Value.Blob = r.Value.Blob[:63] }), keys: keys, want: appstate.ErrMalformed},
		{name: "no ciphertext at all", op: wire.SyncdMutation_SET, record: tamper(func(r *wire.SyncdRecord) { r.Value.Blob = r.Value.Blob[:48] }), keys: keys, want: appstate.ErrMalformed},
		{name: "not whole blocks", op: wire.SyncdMutation_SET, record: tamper(func(r *wire.SyncdRecord) { r.Value.Blob = append([]byte{0}, r.Value.Blob...) }), keys: keys, want: appstate.ErrMalformed},
		{name: "no value", op: wire.SyncdMutation_SET, record: &wire.SyncdRecord{}, keys: keys, want: appstate.ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := appstate.Decrypt(tt.op, tt.record, tt.keys); !errors.Is(err, tt.want) {
				t.Fatalf("Decrypt() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLTHashIsAnOrderFreeMultiset(t *testing.T) {
	t.Parallel()
	a, b, c := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32), bytes.Repeat([]byte{3}, 32)
	ab, err := appstate.LTHash{}.Add(a, b)
	if err != nil {
		t.Fatal(err)
	}
	ba, err := appstate.LTHash{}.Add(b, a)
	if err != nil {
		t.Fatal(err)
	}
	if ab != ba || ab == (appstate.LTHash{}) {
		t.Fatal("adding is not order free")
	}
	abc, err := ab.Add(c)
	if err != nil {
		t.Fatal(err)
	}
	if back, err := abc.Subtract(c); err != nil || back != ab {
		t.Fatalf("subtract does not undo add: %v", err)
	}
	if twice, err := ab.Add(a); err != nil || twice == ab {
		t.Fatalf("adding the same mac twice counts twice: %v", err)
	}
}

func contactNames(mutations []appstate.Mutation) map[string]string {
	out := map[string]string{}
	for _, m := range mutations {
		if len(m.Index) == 2 && m.Index[0] == "contact" {
			out[m.Index[1]] = m.Value.GetContactAction().GetFullName()
			if m.Operation == wire.SyncdMutation_REMOVE {
				out[m.Index[1]] = "(removed)"
			}
		}
	}
	return out
}

func TestSnapshotThenPatches(t *testing.T) {
	t.Parallel()
	s := server(t)
	name := appstate.CriticalUnblockLow
	for _, c := range []fakeappstate.Change{
		fakeappstate.Set(contact("Mama Ioana", "Mama"), "contact", "40744444444@s.whatsapp.net"),
		fakeappstate.Set(contact("Bob Builder", "Bob"), "contact", "40722222222@s.whatsapp.net"),
		fakeappstate.Set(contact("Old Friend", "Old"), "contact", "40755555555@s.whatsapp.net"),
	} {
		if _, err := s.Patch(name, c); err != nil {
			t.Fatal(err)
		}
	}
	state, mutations, err := appstate.State{}.ApplySnapshot(name, s.Snapshot(name), s.KeyFor)
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != 3 || len(state.MACs) != 3 || len(mutations) != 3 || contactNames(mutations)["40744444444@s.whatsapp.net"] != "Mama Ioana" {
		t.Fatalf("after the snapshot: v%d, %d macs, %v", state.Version, len(state.MACs), contactNames(mutations))
	}
	patch, err := s.Patch(name,
		fakeappstate.Set(contact("Mama", "Mama"), "contact", "40744444444@s.whatsapp.net"),
		fakeappstate.Remove("contact", "40755555555@s.whatsapp.net"),
		fakeappstate.Set(contact("Carol Mihai", "Carol"), "contact", "40733333333@s.whatsapp.net"),
	)
	if err != nil {
		t.Fatal(err)
	}
	next, mutations, err := state.ApplyPatch(name, patch, s.KeyFor)
	if err != nil {
		t.Fatal(err)
	}
	names := contactNames(mutations)
	if next.Version != 4 || len(next.MACs) != 3 || names["40744444444@s.whatsapp.net"] != "Mama" || names["40755555555@s.whatsapp.net"] != "(removed)" || names["40733333333@s.whatsapp.net"] != "Carol Mihai" {
		t.Fatalf("after the patch: v%d, %d macs, %v", next.Version, len(next.MACs), names)
	}
	if state.Version != 3 || len(state.MACs) != 3 {
		t.Fatal("applying a patch changed the state it was applied to")
	}
	fresh, _, err := appstate.State{}.ApplySnapshot(name, s.Snapshot(name), s.KeyFor)
	if err != nil || fresh.Hash != next.Hash || fresh.Version != next.Version {
		t.Fatalf("a fresh snapshot and snapshot plus patch disagree: %v", err)
	}
}

func TestRefusingBadSnapshotsAndPatches(t *testing.T) {
	t.Parallel()
	s := server(t)
	name := appstate.Regular
	if _, err := s.Patch(name, fakeappstate.Set(contact("A", "A"), "contact", "1@s.whatsapp.net")); err != nil {
		t.Fatal(err)
	}
	snapshot := s.Snapshot(name)
	none := func([]byte) (appstate.Keys, bool) { return appstate.Keys{}, false }
	state, _, err := appstate.State{}.ApplySnapshot(name, snapshot, s.KeyFor)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := s.Patch(name, fakeappstate.Set(contact("B", "B"), "contact", "2@s.whatsapp.net"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		apply func() (appstate.State, error)
		want  error
	}{
		{name: "snapshot without its key", apply: func() (appstate.State, error) {
			st, _, err := appstate.State{}.ApplySnapshot(name, snapshot, none)
			return st, err
		}, want: appstate.ErrMissingKey},
		{name: "snapshot of another collection", apply: func() (appstate.State, error) {
			st, _, err := appstate.State{}.ApplySnapshot(appstate.RegularHigh, snapshot, s.KeyFor)
			return st, err
		}, want: appstate.ErrSnapshotMAC},
		{name: "snapshot with a forged mac", apply: func() (appstate.State, error) {
			forged := proto.CloneOf(snapshot)
			forged.Mac[0] ^= 1
			st, _, err := appstate.State{}.ApplySnapshot(name, forged, s.KeyFor)
			return st, err
		}, want: appstate.ErrSnapshotMAC},
		{name: "snapshot missing a record", apply: func() (appstate.State, error) {
			short := proto.CloneOf(snapshot)
			short.Records = nil
			st, _, err := appstate.State{}.ApplySnapshot(name, short, s.KeyFor)
			return st, err
		}, want: appstate.ErrSnapshotMAC},
		{name: "patch without its key", apply: func() (appstate.State, error) {
			st, _, err := state.ApplyPatch(name, patch, none)
			return st, err
		}, want: appstate.ErrMissingKey},
		{name: "patch with a forged patch mac", apply: func() (appstate.State, error) {
			forged := proto.CloneOf(patch)
			forged.PatchMac[0] ^= 1
			st, _, err := state.ApplyPatch(name, forged, s.KeyFor)
			return st, err
		}, want: appstate.ErrPatchMAC},
		{name: "patch on a state it does not follow", apply: func() (appstate.State, error) {
			st, _, err := appstate.State{}.ApplyPatch(name, patch, s.KeyFor)
			return st, err
		}, want: appstate.ErrSnapshotMAC},
		{name: "patch with a forged snapshot mac", apply: func() (appstate.State, error) {
			forged := proto.CloneOf(patch)
			forged.SnapshotMac[0] ^= 1
			st, _, err := state.ApplyPatch(name, forged, s.KeyFor)
			return st, err
		}, want: appstate.ErrSnapshotMAC},
		{name: "patch with a tampered record", apply: func() (appstate.State, error) {
			forged := proto.CloneOf(patch)
			forged.Mutations[0].Record.Value.Blob[20] ^= 1
			st, _, err := state.ApplyPatch(name, forged, s.KeyFor)
			return st, err
		}, want: appstate.ErrValueMAC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := tt.apply(); !errors.Is(err, tt.want) || got.Version != 0 && got.Version != state.Version {
				t.Fatalf("got v%d, %v; want %v", got.Version, err, tt.want)
			}
		})
	}
	other := server(t)
	if _, err := other.Patch(name, fakeappstate.Set(contact("X", "X"), "contact", "9@s.whatsapp.net")); err != nil {
		t.Fatal(err)
	}
	removal, err := other.Patch(name, fakeappstate.Remove("contact", "9@s.whatsapp.net"))
	if err != nil {
		t.Fatal(err)
	}
	if st, _, err := (appstate.State{}).ApplyPatch(name, removal, other.KeyFor); err != nil || st.Version != 2 || len(st.MACs) != 0 {
		t.Fatalf("removing a record never seen is skipped, like the official client: v%d, %v", st.Version, err)
	}
}

func TestSyncRequests(t *testing.T) {
	t.Parallel()
	request := appstate.SyncRequest(map[string]uint64{appstate.Regular: 12}, []string{appstate.CriticalUnblockLow, appstate.Regular})
	xmlns, _ := request.Attr("xmlns").Text()
	kind, _ := request.Attr("type").Text()
	sync, ok := request.Child("sync")
	if request.Tag != "iq" || xmlns != "w:sync:app:state" || kind != "set" || !ok || len(sync.Children) != 2 {
		t.Fatalf("request = %s", request)
	}
	for i, want := range []struct{ name, snapshot, version string }{
		{appstate.CriticalUnblockLow, "true", "0"},
		{appstate.Regular, "false", "12"},
	} {
		c := sync.Children[i]
		keys := make([]string, len(c.Attrs))
		for j, a := range c.Attrs {
			keys[j] = a.Key
		}
		name, _ := c.Attr("name").Text()
		snapshot, _ := c.Attr("return_snapshot").Text()
		version, _ := c.Attr("version").Text()
		if name != want.name || snapshot != want.snapshot || version != want.version || keys[0] != "name" || keys[1] != "return_snapshot" || keys[2] != "version" {
			t.Fatalf("collection %d = %s", i, c)
		}
	}
	if got := len(appstate.Collections()); got != 5 {
		t.Fatalf("%d collections", got)
	}
}

func TestParsingSyncReplies(t *testing.T) {
	t.Parallel()
	patch, err := proto.Marshal(&wire.SyncdPatch{Version: &wire.SyncdVersion{Version: new(uint64(8))}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := proto.Marshal(&wire.ExternalBlobReference{DirectPath: new("/v/snap")})
	if err != nil {
		t.Fatal(err)
	}
	attr := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
	collection := func(attrs []node.Attr, children ...node.Node) node.Node {
		return node.Node{Tag: "collection", Attrs: attrs, Children: children}
	}
	failure := func(code string) node.Node { return node.Node{Tag: "error", Attrs: []node.Attr{attr("code", code)}} }
	reply := node.Node{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync", Children: []node.Node{
		collection([]node.Attr{attr("name", appstate.CriticalBlock), attr("version", "8")}, node.Node{Tag: "patches", Children: []node.Node{{Tag: "patch", Bytes: patch}, {Tag: "other"}}}),
		collection([]node.Attr{attr("name", appstate.CriticalUnblockLow), attr("version", "3"), attr("has_more_patches", "true")}, node.Node{Tag: "snapshot", Bytes: snapshot}),
		collection([]node.Attr{attr("name", appstate.RegularHigh), attr("type", "error")}, failure("409")),
		collection([]node.Attr{attr("name", appstate.Regular), attr("type", "error"), attr("has_more_patches", "true")}, failure("409")),
		collection([]node.Attr{attr("name", appstate.RegularLow), attr("type", "error")}, failure("400")),
		collection([]node.Attr{attr("name", appstate.RegularLow), attr("type", "error")}, failure("404")),
		collection([]node.Attr{attr("name", appstate.RegularLow), attr("type", "error")}, failure("500")),
		{Tag: "ignored"},
	}}}}
	got, err := appstate.ParseSync(reply)
	if err != nil {
		t.Fatal(err)
	}
	want := []appstate.Outcome{appstate.Success, appstate.SuccessHasMore, appstate.Conflict, appstate.ConflictHasMore, appstate.ErrorFatal, appstate.ErrorFatal, appstate.ErrorRetry}
	if len(got) != len(want) {
		t.Fatalf("%d collections", len(got))
	}
	for i, w := range want {
		if got[i].Outcome != w {
			t.Errorf("collection %d outcome %d, want %d", i, got[i].Outcome, w)
		}
	}
	if got[0].Version != 8 || len(got[0].Patches) != 1 || got[0].Patches[0].GetVersion().GetVersion() != 8 || got[1].Snapshot.GetDirectPath() != "/v/snap" || got[1].Version != 3 || got[2].Version != 0 {
		t.Fatalf("parsed = %+v", got)
	}
	bad := []node.Node{
		{Tag: "iq", Attrs: []node.Attr{attr("type", "error")}},
		{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}},
		{Tag: "message", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync"}}},
		{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync", Children: []node.Node{collection([]node.Attr{attr("name", "made_up")})}}}},
		{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync", Children: []node.Node{collection([]node.Attr{attr("name", appstate.Regular), attr("version", "x")})}}}},
		{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync", Children: []node.Node{collection([]node.Attr{attr("name", appstate.Regular)}, node.Node{Tag: "patches", Children: []node.Node{{Tag: "patch", Bytes: []byte{0xff}}}})}}}},
		{Tag: "iq", Attrs: []node.Attr{attr("type", "result")}, Children: []node.Node{{Tag: "sync", Children: []node.Node{collection([]node.Attr{attr("name", appstate.Regular)}, node.Node{Tag: "snapshot", Bytes: []byte{0xff}})}}}},
	}
	for i, b := range bad {
		if _, err := appstate.ParseSync(b); !errors.Is(err, appstate.ErrResponse) {
			t.Errorf("bad reply %d: %v", i, err)
		}
	}
}
