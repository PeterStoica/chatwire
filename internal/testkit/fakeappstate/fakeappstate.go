package fakeappstate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

type Host func(name string, plain []byte) (*wire.ExternalBlobReference, error)

type Server struct {
	KeyID []byte
	Keys  appstate.Keys
	host  Host

	mu          sync.Mutex
	collections map[string]*collection
	Requests    int
	MoreFor     int
	ConflictFor int
}

type collection struct {
	state   appstate.State
	records map[string]*wire.SyncdRecord
	patches []*wire.SyncdPatch
}

func New(keyID, keyData []byte, host Host) (*Server, error) {
	keys, err := appstate.Expand(keyData)
	if err != nil {
		return nil, err
	}
	return &Server{KeyID: keyID, Keys: keys, host: host, collections: map[string]*collection{}}, nil
}

func (s *Server) KeyFor(id []byte) (appstate.Keys, bool) {
	return s.Keys, slices.Equal(id, s.KeyID)
}

func (s *Server) collection(name string) *collection {
	c := s.collections[name]
	if c == nil {
		c = &collection{state: appstate.State{MACs: map[string][]byte{}}, records: map[string]*wire.SyncdRecord{}}
		s.collections[name] = c
	}
	return c
}

type Change struct {
	Operation wire.SyncdMutation_SyncdOperation
	Index     []string
	Value     *wire.SyncActionValue
}

func Set(value *wire.SyncActionValue, index ...string) Change {
	return Change{Operation: wire.SyncdMutation_SET, Index: index, Value: value}
}

func Remove(index ...string) Change {
	return Change{Operation: wire.SyncdMutation_REMOVE, Index: index, Value: &wire.SyncActionValue{}}
}

func (s *Server) Patch(name string, changes ...Change) (*wire.SyncdPatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.collection(name)
	patch := &wire.SyncdPatch{KeyId: &wire.KeyId{Id: s.KeyID}}
	var valueMACs [][]byte
	next := appstate.State{Version: c.state.Version + 1, Hash: c.state.Hash, MACs: maps.Clone(c.state.MACs)}
	for _, change := range changes {
		record, err := appstate.Encrypt(rand.Reader, change.Operation, change.Index, change.Value, 3, s.KeyID, s.Keys)
		if err != nil {
			return nil, err
		}
		index := hex.EncodeToString(record.GetIndex().GetBlob())
		valueMAC := record.GetValue().GetBlob()[len(record.GetValue().GetBlob())-32:]
		if old, ok := next.MACs[index]; ok {
			if next.Hash, err = next.Hash.Subtract(old); err != nil {
				return nil, err
			}
		}
		if change.Operation == wire.SyncdMutation_SET {
			if next.Hash, err = next.Hash.Add(valueMAC); err != nil {
				return nil, err
			}
			next.MACs[index] = valueMAC
			c.records[index] = record
		} else {
			delete(next.MACs, index)
			delete(c.records, index)
		}
		patch.Mutations = append(patch.Mutations, &wire.SyncdMutation{Operation: change.Operation.Enum(), Record: record})
		valueMACs = append(valueMACs, valueMAC)
	}
	patch.Version = &wire.SyncdVersion{Version: new(next.Version)}
	patch.SnapshotMac = appstate.SnapshotMAC(s.Keys, next.Hash, next.Version, name)
	patch.PatchMac = appstate.PatchMAC(s.Keys, patch.SnapshotMac, valueMACs, next.Version, name)
	c.state = next
	c.patches = append(c.patches, patch)
	return patch, nil
}

func (s *Server) Snapshot(name string) *wire.SyncdSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.collection(name)
	snapshot := &wire.SyncdSnapshot{
		Version: &wire.SyncdVersion{Version: new(c.state.Version)},
		Mac:     appstate.SnapshotMAC(s.Keys, c.state.Hash, c.state.Version, name),
		KeyId:   &wire.KeyId{Id: s.KeyID},
	}
	for _, index := range sortedKeys(c.records) {
		snapshot.Records = append(snapshot.Records, c.records[index])
	}
	return snapshot
}

func sortedKeys(records map[string]*wire.SyncdRecord) []string {
	keys := make([]string, 0, len(records))
	for k := range records {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (s *Server) Handle(request node.Node) (node.Node, error) {
	s.mu.Lock()
	s.Requests++
	more, conflict := s.Requests <= s.MoreFor, s.Requests <= s.ConflictFor
	s.mu.Unlock()
	sync, _ := request.Child("sync")
	var collections []node.Node
	for _, c := range sync.Children {
		name, _ := c.Attr("name").Text()
		raw, _ := c.Attr("version").Text()
		since, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return node.Node{}, fmt.Errorf("fakeappstate: version %q", raw)
		}
		answer, err := s.answer(name, since)
		if err != nil {
			return node.Node{}, err
		}
		if more {
			answer.Attrs = append(answer.Attrs, node.Attr{Key: "has_more_patches", Value: node.Text("true")})
		}
		if conflict {
			answer.Attrs = append(answer.Attrs, node.Attr{Key: "type", Value: node.Text("error")})
			answer.Children = append(answer.Children, node.Node{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("409")}}})
		}
		collections = append(collections, answer)
	}
	return node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: request.Attr("id")}, {Key: "type", Value: node.Text("result")}},
		Children: []node.Node{{Tag: "sync", Children: collections}}}, nil
}

func (s *Server) answer(name string, since uint64) (node.Node, error) {
	s.mu.Lock()
	c := s.collection(name)
	version, patches := c.state.Version, slices.Clone(c.patches)
	s.mu.Unlock()
	out := node.Node{Tag: "collection", Attrs: []node.Attr{{Key: "name", Value: node.Text(name)}, {Key: "version", Value: node.Text(strconv.FormatUint(version, 10))}}}
	if since == 0 {
		raw, err := proto.Marshal(s.Snapshot(name))
		if err != nil {
			return node.Node{}, err
		}
		reference, err := s.host(name, raw)
		if err != nil {
			return node.Node{}, err
		}
		encoded, err := proto.Marshal(reference)
		if err != nil {
			return node.Node{}, err
		}
		out.Children = append(out.Children, node.Node{Tag: "snapshot", Bytes: encoded})
		return out, nil
	}
	var encoded []node.Node
	for _, p := range patches {
		if p.GetVersion().GetVersion() <= since {
			continue
		}
		raw, err := proto.Marshal(p)
		if err != nil {
			return node.Node{}, err
		}
		encoded = append(encoded, node.Node{Tag: "patch", Bytes: raw})
	}
	if len(encoded) > 0 {
		out.Children = append(out.Children, node.Node{Tag: "patches", Children: encoded})
	}
	return out, nil
}
