package messenger

import (
	"context"
	"time"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (m *Messenger) SyncState(ctx context.Context, collection string) (appstate.State, error) {
	saved, err := m.store.SyncState(ctx, collection)
	if err != nil {
		return appstate.State{}, err
	}
	state := appstate.State{Version: saved.Version, MACs: saved.MACs}
	copy(state.Hash[:], saved.Hash)
	return state, nil
}

func (m *Messenger) SaveSync(ctx context.Context, collection string, state appstate.State, mutations []appstate.Mutation) error {
	changes := chatFlagsFrom(mutations, m.link.Now())
	changes.Contacts = contactsFrom(mutations)
	return m.store.SaveSync(ctx, collection, store.SyncState{Version: state.Version, Hash: state.Hash[:], MACs: state.MACs}, changes)
}

func chatFlagsFrom(mutations []appstate.Mutation, now time.Time) store.SyncChanges {
	changes := store.SyncChanges{Pins: map[node.JID]time.Time{}, Archives: map[node.JID]bool{}, Mutes: map[node.JID]store.Mute{}}
	for _, mutation := range mutations {
		if len(mutation.Index) < 2 || mutation.Operation != wire.SyncdMutation_SET {
			continue
		}
		jid, err := node.ParseJID(mutation.Index[1])
		if err != nil {
			continue
		}
		switch mutation.Index[0] {
		case "pin_v1":
			pinFrom(changes, jid, mutation.Value)
		case "archive":
			if archive := mutation.Value.GetArchiveChatAction(); archive != nil && archive.Archived != nil {
				changes.Archives[jid] = archive.GetArchived()
			}
		case "mute":
			muteFrom(changes, jid, mutation.Value.GetMuteAction(), now)
		}
	}
	return changes
}

func pinFrom(changes store.SyncChanges, jid node.JID, value *wire.SyncActionValue) {
	pin := value.GetPinAction()
	switch {
	case pin == nil || pin.Pinned == nil:
	case pin.GetPinned():
		changes.Pins[jid] = time.UnixMilli(max(value.GetTimestamp(), 1))
	default:
		changes.Pins[jid] = time.Time{}
	}
}

func muteFrom(changes store.SyncChanges, jid node.JID, mute *wire.SyncActionValue_MuteAction, now time.Time) {
	if mute != nil && mute.Muted != nil && (!mute.GetMuted() || mute.MuteEndTimestamp != nil) {
		changes.Mutes[jid] = muteOf(mute.GetMuteEndTimestamp(), now)
	}
}

func muteOf(endMs int64, now time.Time) store.Mute {
	switch {
	case endMs < 0 && endMs >= -1000:
		return store.MuteForever
	case endMs < now.UnixMilli():
		return 0
	}
	return store.Mute(endMs / 1000)
}

func contactsFrom(mutations []appstate.Mutation) []store.Contact {
	var out []store.Contact
	for _, mutation := range mutations {
		if len(mutation.Index) < 2 || mutation.Index[0] != "contact" {
			continue
		}
		jid, err := node.ParseJID(mutation.Index[1])
		if err != nil {
			continue
		}
		contact := store.Contact{JID: jid}
		if mutation.Operation == wire.SyncdMutation_SET {
			action := mutation.Value.GetContactAction()
			contact.Name = store.Name{Contact: action.GetFullName(), First: action.GetFirstName()}
		}
		out = append(out, contact)
	}
	return out
}
