package messenger

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestWhoMayEditDeleteAndReact(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobPhone := node.JID{User: bob.User, Device: 3, Server: bob.Server}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	carolLID := node.JID{User: "88333", Server: node.ServerLID}
	group := node.JID{User: "120363000000000081", Server: node.ServerGroup}
	m := New(linkflow.Config{Now: time.Now}, nil, nil, st)
	m.state = &client.State{Linked: linkflow.Linked{Account: pairing.Account{JID: me}}}
	if err := st.Apply(ctx, store.Changes{LIDs: map[node.JID]node.JID{carolLID: carol}}); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1790000000, 0)
	say := func(id string, chat, author node.JID, text string) {
		m.received(client.Received{ID: id, Chat: chat, Author: author, Time: at, Edit: message.EditNone, Message: &wire.Message{Conversation: new(text)}})
	}
	protocol := func(chat, author node.JID, edit message.Edit, target string, kind wire.Message_ProtocolMessage_Type, edited *wire.Message) {
		m.received(client.Received{ID: "p-" + target, Chat: chat, Author: author, Time: at.Add(time.Minute), Edit: edit, Message: &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
			Key: &wire.MessageKey{Id: new(target)}, Type: kind.Enum(), EditedMessage: edited, TimestampMs: new(at.Add(2 * time.Minute).UnixMilli()),
		}}})
	}
	text := func(s string) *wire.Message { return &wire.Message{Conversation: new(s)} }
	say("b1", bob, bobPhone, "helo")
	say("g1", group, bob, "bob in the group")
	say("g2", group, carol, "carol in the group")
	say("g3", group, bob, "rude words")
	say("m1", bob, me, "on my way")
	say("b2", bob, bob, "caption-less text")

	protocol(bob, bob, message.EditMessage, "b1", wire.Message_ProtocolMessage_MESSAGE_EDIT, text("hello"))
	protocol(group, carol, message.EditMessage, "g1", wire.Message_ProtocolMessage_MESSAGE_EDIT, text("carol rewrote bob"))
	protocol(group, carolLID, message.EditMessage, "g2", wire.Message_ProtocolMessage_MESSAGE_EDIT, text("carol from her lid"))
	protocol(group, carol, message.EditSenderRevoke, "g1", wire.Message_ProtocolMessage_REVOKE, nil)
	protocol(group, carol, message.EditAdminRevoke, "g3", wire.Message_ProtocolMessage_REVOKE, nil)
	protocol(bob, carol, message.EditAdminRevoke, "b2", wire.Message_ProtocolMessage_REVOKE, nil)
	protocol(bob, bob, message.EditMessage, "b2", wire.Message_ProtocolMessage_MESSAGE_EDIT, &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("x")}})
	protocol(bob, bob, message.EditMessage, "nope", wire.Message_ProtocolMessage_MESSAGE_EDIT, text("nothing to edit"))
	protocol(bob, bob, message.EditSenderRevoke, "nope", wire.Message_ProtocolMessage_REVOKE, nil)
	m.received(client.Received{ID: "dsm", Chat: bob, Author: node.JID{User: me.User, Device: 2, Server: me.Server}, Time: at, Edit: message.EditMessage, Message: &wire.Message{
		DeviceSentMessage: &wire.Message_DeviceSentMessage{Message: &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
			Key: &wire.MessageKey{Id: new("m1")}, Type: wire.Message_ProtocolMessage_MESSAGE_EDIT.Enum(), EditedMessage: text("almost there"),
		}}},
	}})
	m.received(client.Received{ID: "r1", Chat: bob, Author: bobPhone, Time: at, Message: &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{
		Key: &wire.MessageKey{Id: new("m1")}, Text: new("👍"), SenderTimestampMs: new(at.Add(5 * time.Minute).UnixMilli()),
	}}})
	m.received(client.Received{ID: "r2", Chat: bob, Author: bob, Time: at, Message: &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Text: new("🔥")}}})
	if err := m.Connection().StoreErr; err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		chat    node.JID
		id      string
		text    string
		edited  bool
		revoked bool
	}{
		{bob, "b1", "hello", true, false},
		{group, "g1", "bob in the group", false, false},
		{group, "g2", "carol from her lid", true, false},
		{group, "g3", "", false, true},
		{bob, "b2", "caption-less text", false, false},
		{bob, "m1", "almost there", true, false},
	} {
		got, ok, err := st.MessageIn(ctx, tt.chat, tt.id)
		if err != nil || !ok || store.Text(got.Message) != tt.text || !got.Edited.IsZero() != tt.edited || got.Revoked != tt.revoked {
			t.Errorf("%s = %q edited %v revoked %v, want %q %v %v", tt.id, store.Text(got.Message), !got.Edited.IsZero(), got.Revoked, tt.text, tt.edited, tt.revoked)
		}
	}
	mine, _, err := st.MessageIn(ctx, bob, "m1")
	if err != nil || len(mine.Reactions) != 1 || mine.Reactions[0].By != bob || mine.Reactions[0].Emoji != "👍" || !mine.Reactions[0].Time.Equal(at.Add(5*time.Minute)) {
		t.Fatalf("reactions on m1 = %+v, %v", mine.Reactions, err)
	}
	if edited, _, _ := st.MessageIn(ctx, bob, "b1"); !edited.Edited.Equal(at.Add(2 * time.Minute)) {
		t.Fatalf("the edit time comes from the edit: %v", edited.Edited)
	}
	for _, id := range []string{"p-b1", "p-g1", "r1", "r2", "dsm"} {
		if _, stored, _ := st.MessageIn(ctx, bob, id); stored {
			t.Errorf("protocol message %s was stored as a chat message", id)
		}
	}
}

func TestSentAtFallsBackToTheStanzaTime(t *testing.T) {
	t.Parallel()
	stanza := time.Unix(1790000000, 0)
	for _, tt := range []struct {
		ms   int64
		want time.Time
	}{
		{0, stanza},
		{-5, stanza},
		{1, time.UnixMilli(1)},
		{1790000123456, time.UnixMilli(1790000123456)},
	} {
		if got := sentAt(tt.ms, stanza); !got.Equal(tt.want) {
			t.Errorf("sentAt(%d) = %v, want %v", tt.ms, got, tt.want)
		}
	}
}

func TestReceiptsMoveTicksAndUnreadCounts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobPhone := node.JID{User: bob.User, Device: 2, Server: bob.Server}
	group := node.JID{User: "120363000000000111", Server: node.ServerGroup}
	m := New(linkflow.Config{Now: time.Now}, nil, nil, st)
	m.state = &client.State{Linked: linkflow.Linked{Account: pairing.Account{JID: me}}}
	at := time.Unix(1790000000, 0)
	for _, r := range []client.Received{
		{ID: "o1", Chat: bob, Author: me, Time: at, Message: &wire.Message{Conversation: new("mine")}},
		{ID: "o2", Chat: bob, Author: me, Time: at, Message: &wire.Message{Conversation: new("mine too")}},
		{ID: "g1", Chat: group, Author: me, Time: at, Message: &wire.Message{Conversation: new("to the group")}},
		{ID: "i1", Chat: bob, Author: bobPhone, Time: at, Message: &wire.Message{Conversation: new("theirs")}},
		{ID: "i2", Chat: bob, Author: bob, Time: at, Message: &wire.Message{Conversation: new("theirs too")}},
	} {
		m.received(r)
	}
	unread := func(chat node.JID) int {
		chats, err := st.Chats(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chats {
			if c.JID == chat {
				return c.Unread
			}
		}
		return -1
	}
	if got := unread(bob); got != 2 {
		t.Fatalf("unread before any receipt = %d", got)
	}
	for _, r := range []message.Receipt{
		{From: bobPhone, Ack: message.AckRead, IDs: []string{"o1"}},
		{From: bob, Ack: message.AckDelivered, IDs: []string{"o2"}},
		{From: group, Participant: bob, Ack: message.AckRead, IDs: []string{"g1"}},
		{From: bob, Ack: message.AckInactive, IDs: []string{"o2"}},
		{From: bob, Ack: message.AckDelivered, Self: true, IDs: []string{"i1"}},
	} {
		m.receipt(r)
	}
	if got := unread(bob); got != 2 {
		t.Fatalf("a delivery on our own phone must not clear the count: %d", got)
	}
	statuses := map[string]store.Status{}
	for _, chat := range []node.JID{bob, group} {
		found, err := st.Messages(ctx, store.Query{Chat: chat, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range found {
			statuses[msg.ID] = msg.Status
		}
	}
	if statuses["o1"] != store.StatusRead || statuses["o2"] != store.StatusDelivered || statuses["g1"] != store.StatusSent {
		t.Fatalf("statuses = %v", statuses)
	}
	m.receipt(message.Receipt{From: bob, Ack: message.AckPlayed, Self: true, IDs: []string{"i2"}})
	if got := unread(bob); got != 0 {
		t.Fatalf("reading on our own phone must clear the count: %d", got)
	}
	m.received(client.Received{ID: "i3", Chat: bob, Author: bob, Time: at, Message: &wire.Message{Conversation: new("again")}})
	m.receipt(message.Receipt{From: bobPhone, Ack: message.AckRead, Self: true, IDs: []string{"i3"}})
	m.receipt(message.Receipt{From: bob, Ack: message.AckPlayed, IDs: []string{"o2"}})
	if got := unread(bob); got != 0 {
		t.Fatalf("read-self from a device address must clear the chat too: %d", got)
	}
	if found, _, _ := st.MessageIn(ctx, bob, "o2"); found.Status != store.StatusPlayed {
		t.Fatalf("played = %v", found.Status)
	}
	if err := m.Connection().StoreErr; err != nil {
		t.Fatal(err)
	}
}

func TestOnlyTheCurrentConnectionSavesTheLink(t *testing.T) {
	t.Parallel()
	var saved []client.State
	m := New(linkflow.Config{Now: time.Now}, nil, func(s client.State) error {
		saved = append(saved, s)
		return nil
	}, nil)
	m.state = &client.State{}
	old, current := m.saver(), m.saver()
	if err := old(client.State{NextPreKeyID: 1}); err != nil || len(saved) != 0 {
		t.Fatalf("a previous connection saved over the current one: %v, %d saves", err, len(saved))
	}
	if err := current(client.State{NextPreKeyID: 2}); err != nil || len(saved) != 1 || saved[0].NextPreKeyID != 2 {
		t.Fatalf("the current connection did not save: %v, %+v", err, saved)
	}
	m.loggedOut(errors.New("unlinked"))
	if err := current(client.State{NextPreKeyID: 3}); err != nil || len(saved) != 1 {
		t.Fatalf("a save after logging out brought the link back: %v, %d saves", err, len(saved))
	}
}
