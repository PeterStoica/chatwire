package appstate

import (
	"crypto/hmac"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	CriticalBlock      = "critical_block"
	CriticalUnblockLow = "critical_unblock_low"
	RegularHigh        = "regular_high"
	Regular            = "regular"
	RegularLow         = "regular_low"
)

var (
	ErrMissingKey  = errors.New("appstate: sync key not shared yet")
	ErrSnapshotMAC = errors.New("appstate: snapshot mac mismatch")
	ErrPatchMAC    = errors.New("appstate: patch mac mismatch")
	ErrResponse    = errors.New("appstate: unexpected sync reply")
)

func Collections() []string {
	return []string{CriticalBlock, CriticalUnblockLow, RegularHigh, Regular, RegularLow}
}

type State struct {
	Version uint64
	Hash    LTHash
	MACs    map[string][]byte
}

func (s State) clone() State {
	out := s
	out.MACs = maps.Clone(s.MACs)
	if out.MACs == nil {
		out.MACs = map[string][]byte{}
	}
	return out
}

type MissingKey struct {
	ID []byte
}

func (k MissingKey) Error() string {
	return fmt.Sprintf("%v: %x", ErrMissingKey, k.ID)
}

func (k MissingKey) Is(target error) bool {
	return target == ErrMissingKey
}

type KeyFor func(id []byte) (Keys, bool)

func (s State) ApplySnapshot(name string, snapshot *wire.SyncdSnapshot, keyFor KeyFor) (State, []Mutation, int, error) {
	keys, ok := keyFor(snapshot.GetKeyId().GetId())
	if !ok {
		return s, nil, 0, MissingKey{ID: snapshot.GetKeyId().GetId()}
	}
	next := State{Version: snapshot.GetVersion().GetVersion(), MACs: map[string][]byte{}}
	mutations := make([]Mutation, 0, len(snapshot.GetRecords()))
	skipped := 0
	for _, record := range snapshot.GetRecords() {
		m, readable, err := open(wire.SyncdMutation_SET, record, keyFor)
		if err != nil {
			return s, nil, 0, err
		}
		if err := set(&next, m); err != nil {
			return s, nil, 0, err
		}
		if !readable {
			skipped++
			continue
		}
		mutations = append(mutations, m)
	}
	if !hmac.Equal(SnapshotMAC(keys, next.Hash, next.Version, name), snapshot.GetMac()) {
		return s, nil, 0, fmt.Errorf("%w: %s v%d", ErrSnapshotMAC, name, next.Version)
	}
	return next, mutations, skipped, nil
}

func (s State) ApplyPatch(name string, patch *wire.SyncdPatch, keyFor KeyFor) (State, []Mutation, int, error) {
	keys, ok := keyFor(patch.GetKeyId().GetId())
	if !ok {
		return s, nil, 0, MissingKey{ID: patch.GetKeyId().GetId()}
	}
	next := s.clone()
	next.Version = patch.GetVersion().GetVersion()
	mutations := make([]Mutation, 0, len(patch.GetMutations()))
	valueMACs := make([][]byte, 0, len(patch.GetMutations()))
	skipped := 0
	for _, pm := range patch.GetMutations() {
		m, readable, err := open(pm.GetOperation(), pm.GetRecord(), keyFor)
		if err != nil {
			return s, nil, 0, err
		}
		if m.Operation == wire.SyncdMutation_REMOVE {
			err = remove(&next, m)
		} else {
			err = set(&next, m)
		}
		if err != nil {
			return s, nil, 0, err
		}
		valueMACs = append(valueMACs, m.ValueMAC)
		if !readable {
			skipped++
			continue
		}
		mutations = append(mutations, m)
	}
	if !hmac.Equal(SnapshotMAC(keys, next.Hash, next.Version, name), patch.GetSnapshotMac()) {
		return s, nil, 0, fmt.Errorf("%w: %s after patch v%d", ErrSnapshotMAC, name, next.Version)
	}
	if !hmac.Equal(PatchMAC(keys, patch.GetSnapshotMac(), valueMACs, next.Version, name), patch.GetPatchMac()) {
		return s, nil, 0, fmt.Errorf("%w: %s v%d", ErrPatchMAC, name, next.Version)
	}
	return next, mutations, skipped, nil
}

func open(op wire.SyncdMutation_SyncdOperation, record *wire.SyncdRecord, keyFor KeyFor) (Mutation, bool, error) {
	blob := record.GetValue().GetBlob()
	folded := Mutation{Operation: op, IndexMAC: record.GetIndex().GetBlob()}
	if len(blob) >= macSize {
		folded.ValueMAC = blob[len(blob)-macSize:]
	}
	if len(folded.IndexMAC) == 0 || len(folded.ValueMAC) != macSize {
		return Mutation{}, false, fmt.Errorf("%w: a record without its index or value mac", ErrMalformed)
	}
	keys, ok := keyFor(record.GetKeyId().GetId())
	if !ok {
		return folded, false, nil
	}
	m, err := Decrypt(op, record, keys)
	if err != nil {
		return folded, false, nil
	}
	return m, true, nil
}

func set(s *State, m Mutation) error {
	index := hex.EncodeToString(m.IndexMAC)
	var err error
	if old, ok := s.MACs[index]; ok {
		if s.Hash, err = s.Hash.Subtract(old); err != nil {
			return err
		}
	}
	if s.Hash, err = s.Hash.Add(m.ValueMAC); err != nil {
		return err
	}
	s.MACs[index] = m.ValueMAC
	return nil
}

func remove(s *State, m Mutation) error {
	index := hex.EncodeToString(m.IndexMAC)
	old, ok := s.MACs[index]
	if !ok {
		return nil
	}
	var err error
	if s.Hash, err = s.Hash.Subtract(old); err != nil {
		return err
	}
	delete(s.MACs, index)
	return nil
}

func SyncRequest(versions map[string]uint64, names []string) node.Node {
	collections := make([]node.Node, 0, len(names))
	for _, name := range names {
		version := versions[name]
		collections = append(collections, node.Node{Tag: "collection", Attrs: []node.Attr{
			{Key: "name", Value: node.Text(name)},
			{Key: "return_snapshot", Value: node.Text(strconv.FormatBool(version == 0))},
			{Key: "version", Value: node.Text(strconv.FormatUint(version, 10))},
		}})
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "id", Value: node.Value{}}, {Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})},
			{Key: "type", Value: node.Text("set")}, {Key: "xmlns", Value: node.Text("w:sync:app:state")},
		},
		Children: []node.Node{{Tag: "sync", Children: collections}},
	}
}

type Outcome int

const (
	Success Outcome = iota
	SuccessHasMore
	Conflict
	ConflictHasMore
	ErrorRetry
	ErrorFatal
)

type Response struct {
	Name     string
	Outcome  Outcome
	Version  uint64
	Patches  []*wire.SyncdPatch
	Snapshot *wire.ExternalBlobReference
}

func ParseSync(reply node.Node) ([]Response, error) {
	sync, ok := reply.Child("sync")
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "result" || !ok {
		return nil, fmt.Errorf("%w: %s", ErrResponse, reply)
	}
	var out []Response
	for _, c := range sync.Children {
		if c.Tag != "collection" {
			continue
		}
		r, err := parseCollection(c)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseCollection(c node.Node) (Response, error) {
	name, _ := c.Attr("name").Text()
	if !known(name) {
		return Response{}, fmt.Errorf("%w: collection %q", ErrResponse, name)
	}
	r := Response{Name: name, Outcome: outcome(c)}
	if raw, ok := c.Attr("version").Text(); ok {
		version, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return Response{}, fmt.Errorf("%w: version %q", ErrResponse, raw)
		}
		r.Version = version
	}
	if patches, ok := c.Child("patches"); ok {
		for _, p := range patches.Children {
			if p.Tag != "patch" {
				continue
			}
			var patch wire.SyncdPatch
			if err := proto.Unmarshal(p.Bytes, &patch); err != nil {
				return Response{}, fmt.Errorf("%w: patch: %w", ErrResponse, err)
			}
			r.Patches = append(r.Patches, &patch)
		}
	}
	if snapshot, ok := c.Child("snapshot"); ok {
		r.Snapshot = &wire.ExternalBlobReference{}
		if err := proto.Unmarshal(snapshot.Bytes, r.Snapshot); err != nil {
			return Response{}, fmt.Errorf("%w: snapshot: %w", ErrResponse, err)
		}
	}
	return r, nil
}

func known(name string) bool {
	return slices.Contains(Collections(), name)
}

func outcome(c node.Node) Outcome {
	_, more := c.Attr("has_more_patches").Text()
	if kind, _ := c.Attr("type").Text(); kind != "error" {
		if more {
			return SuccessHasMore
		}
		return Success
	}
	failure, _ := c.Child("error")
	switch code, _ := failure.Attr("code").Text(); code {
	case "409":
		if more {
			return ConflictHasMore
		}
		return Conflict
	case "400", "404":
		return ErrorFatal
	default:
		return ErrorRetry
	}
}
