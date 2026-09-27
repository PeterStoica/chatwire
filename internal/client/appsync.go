package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const maxSyncRounds = 8

type AppStateStore interface {
	SyncState(ctx context.Context, collection string) (appstate.State, error)
	SaveSync(ctx context.Context, collection string, state appstate.State, mutations []appstate.Mutation) error
}

func (c *Client) addSyncKeys(share *wire.Message_AppStateSyncKeyShare) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range share.GetKeys() {
		id, data := k.GetKeyId().GetKeyId(), k.GetKeyData().GetKeyData()
		if len(id) == 0 || len(data) == 0 || c.hasSyncKeyLocked(id) {
			continue
		}
		c.state.SyncKeys = append(c.state.SyncKeys, SyncKey{ID: id, Data: data, Timestamp: k.GetKeyData().GetTimestamp()})
	}
}

func (c *Client) hasSyncKeyLocked(id []byte) bool {
	for _, k := range c.state.SyncKeys {
		if string(k.ID) == string(id) {
			return true
		}
	}
	return false
}

func (c *Client) syncKey(id []byte) (appstate.Keys, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name := hex.EncodeToString(id)
	if keys, ok := c.syncKeys[name]; ok {
		return keys, true
	}
	for _, k := range c.state.SyncKeys {
		if string(k.ID) != string(id) {
			continue
		}
		keys, err := appstate.Expand(k.Data)
		if err != nil {
			return appstate.Keys{}, false
		}
		if c.syncKeys == nil {
			c.syncKeys = map[string]appstate.Keys{}
		}
		c.syncKeys[name] = keys
		return keys, true
	}
	return appstate.Keys{}, false
}

func (c *Client) queueAppStateSync(names []string) {
	c.mu.Lock()
	ready := c.cfg.AppState != nil && len(c.state.SyncKeys) > 0
	c.mu.Unlock()
	if !ready {
		return
	}
	if len(names) == 0 {
		names = appstate.Collections()
	}
	c.enqueue(func(ctx context.Context) { c.syncAppState(ctx, names) })
}

func (c *Client) notified(n node.Node) {
	kind, _ := n.Attr("type").Text()
	if kind == "mediaretry" {
		c.retried(n)
		return
	}
	if kind != "server_sync" {
		return
	}
	var names []string
	for _, child := range n.Children {
		if name, ok := child.Attr("name").Text(); child.Tag == "collection" && ok {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		c.queueAppStateSync(names)
	}
}

func (c *Client) syncAppState(ctx context.Context, names []string) {
	pending := names
	for round := 0; len(pending) > 0 && round < maxSyncRounds; round++ {
		next, err := c.syncRound(ctx, pending)
		if err != nil {
			return
		}
		pending = next
	}
}

func (c *Client) syncRound(ctx context.Context, names []string) ([]string, error) {
	states := make(map[string]appstate.State, len(names))
	versions := make(map[string]uint64, len(names))
	for _, name := range names {
		st, err := c.cfg.AppState.SyncState(ctx, name)
		if err != nil {
			return nil, err
		}
		states[name], versions[name] = st, st.Version
	}
	reply, err := c.online.Session.Query(ctx, appstate.SyncRequest(versions, names))
	if err != nil {
		return nil, fmt.Errorf("client: app state sync: %w", err)
	}
	responses, err := appstate.ParseSync(reply)
	if err != nil {
		return nil, err
	}
	var again []string
	for _, r := range responses {
		more, err := c.settle(ctx, r, states[r.Name])
		if err != nil {
			return nil, err
		}
		if more {
			again = append(again, r.Name)
		}
	}
	return again, nil
}

func (c *Client) settle(ctx context.Context, r appstate.Response, st appstate.State) (bool, error) {
	if r.Outcome == appstate.ErrorRetry || r.Outcome == appstate.ErrorFatal {
		return false, nil
	}
	next, mutations, err := c.applySync(ctx, r, st)
	switch {
	case errors.Is(err, appstate.ErrMissingKey):
		return false, nil
	case errors.Is(err, appstate.ErrSnapshotMAC), errors.Is(err, appstate.ErrPatchMAC), errors.Is(err, appstate.ErrValueMAC), errors.Is(err, appstate.ErrIndexMAC):
		if st.Version == 0 {
			return false, nil
		}
		return true, c.cfg.AppState.SaveSync(ctx, r.Name, appstate.State{}, nil)
	case err != nil:
		return false, err
	}
	if err := c.cfg.AppState.SaveSync(ctx, r.Name, next, mutations); err != nil {
		return false, err
	}
	return r.Outcome == appstate.SuccessHasMore || r.Outcome == appstate.ConflictHasMore, nil
}

func (c *Client) applySync(ctx context.Context, r appstate.Response, st appstate.State) (appstate.State, []appstate.Mutation, error) {
	var all []appstate.Mutation
	if r.Snapshot != nil {
		var snapshot wire.SyncdSnapshot
		if err := c.downloadBlob(ctx, r.Snapshot, &snapshot); err != nil {
			return st, nil, err
		}
		next, mutations, err := st.ApplySnapshot(r.Name, &snapshot, c.syncKey)
		if err != nil {
			return st, nil, err
		}
		st, all = next, mutations
	}
	for _, patch := range r.Patches {
		if external := patch.GetExternalMutations(); external != nil {
			var mutations wire.SyncdMutations
			if err := c.downloadBlob(ctx, external, &mutations); err != nil {
				return st, nil, err
			}
			patch.Mutations = mutations.GetMutations()
		}
		next, mutations, err := st.ApplyPatch(r.Name, patch, c.syncKey)
		if err != nil {
			return st, nil, err
		}
		st, all = next, append(all, mutations...)
	}
	return st, all, nil
}

func (c *Client) downloadBlob(ctx context.Context, ref *wire.ExternalBlobReference, into proto.Message) error {
	raw, err := c.Download(ctx, media.Reference{
		Type: media.AppState, DirectPath: ref.GetDirectPath(), MediaKey: ref.GetMediaKey(),
		FileSHA256: ref.GetFileSha256(), FileEncSHA256: ref.GetFileEncSha256(),
	})
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("client: app state blob: %w", err)
	}
	return nil
}
