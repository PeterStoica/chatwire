package message_test

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestTypeOf(t *testing.T) {
	future := func(m *wire.Message) *wire.Message_FutureProofMessage {
		return &wire.Message_FutureProofMessage{Message: m}
	}
	reaction := &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{}}
	secret := func(kind wire.Message_SecretEncryptedMessage_SecretEncType) *wire.Message {
		return &wire.Message{SecretEncryptedMessage: &wire.Message_SecretEncryptedMessage{SecretEncType: kind.Enum()}}
	}
	for _, tt := range []struct {
		name string
		m    *wire.Message
		want string
	}{
		{"conversation", &wire.Message{Conversation: new("")}, message.TypeText},
		{"link preview", &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{MatchedText: new(" x ")}}, message.TypeMedia},
		{"blank matched text", &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{MatchedText: new(" \t")}}, message.TypeText},
		{"image", &wire.Message{ImageMessage: &wire.Message_ImageMessage{}}, message.TypeMedia},
		{"empty", &wire.Message{}, message.TypeMedia},
		{"reaction", reaction, message.TypeReaction},
		{"encrypted reaction", &wire.Message{EncReactionMessage: &wire.Message_EncReactionMessage{}}, message.TypeReaction},
		{"event", &wire.Message{EventMessage: &wire.Message_EventMessage{}}, message.TypeEvent},
		{"event response", &wire.Message{EncEventResponseMessage: &wire.Message_EncEventResponseMessage{}}, message.TypeEvent},
		{"event edit", secret(wire.Message_SecretEncryptedMessage_EVENT_EDIT), message.TypeEvent},
		{"message edit", secret(wire.Message_SecretEncryptedMessage_MESSAGE_EDIT), message.TypeText},
		{"poll edit", secret(wire.Message_SecretEncryptedMessage_POLL_EDIT), message.TypePoll},
		{"poll option", secret(wire.Message_SecretEncryptedMessage_POLL_ADD_OPTION), message.TypePoll},
		{"scheduled", secret(wire.Message_SecretEncryptedMessage_MESSAGE_SCHEDULE), message.TypeMedia},
		{"poll v1", &wire.Message{PollCreationMessage: &wire.Message_PollCreationMessage{}}, message.TypePoll},
		{"poll v2", &wire.Message{PollCreationMessageV2: &wire.Message_PollCreationMessage{}}, message.TypePoll},
		{"poll v3", &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{}}, message.TypePoll},
		{"poll v5", &wire.Message{PollCreationMessageV5: &wire.Message_PollCreationMessage{}}, message.TypePoll},
		{"poll v6", &wire.Message{PollCreationMessageV6: &wire.Message_PollCreationMessage{}}, message.TypePoll},
		{"poll vote", &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{}}, message.TypePoll},
		{"poll results", &wire.Message{PollResultSnapshotMessage: &wire.Message_PollResultSnapshotMessage{}}, message.TypeText},
		{"poll results v3", &wire.Message{PollResultSnapshotMessageV3: &wire.Message_PollResultSnapshotMessage{}}, message.TypeText},
		{"ephemeral reaction", &wire.Message{EphemeralMessage: future(reaction)}, message.TypeReaction},
		{"empty ephemeral", &wire.Message{EphemeralMessage: future(nil)}, message.TypeText},
		{"group mention", &wire.Message{GroupMentionedMessage: future(reaction)}, message.TypeReaction},
		{"bot invoke", &wire.Message{BotInvokeMessage: future(reaction)}, message.TypeReaction},
		{"bot forward", &wire.Message{BotForwardedMessage: future(reaction)}, message.TypeReaction},
		{"device sent", &wire.Message{DeviceSentMessage: &wire.Message_DeviceSentMessage{Message: reaction}}, message.TypeReaction},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := message.TypeOf(tt.m); got != tt.want {
				t.Fatalf("TypeOf() = %q, want %q", got, tt.want)
			}
		})
	}
	textual := []*wire.Message{
		{TemplateButtonReplyMessage: &wire.Message_TemplateButtonReplyMessage{}}, {ProtocolMessage: &wire.Message_ProtocolMessage{}},
		{InteractiveMessage: &wire.Message_InteractiveMessage{}}, {KeepInChatMessage: &wire.Message_KeepInChatMessage{}},
		{RequestPhoneNumberMessage: &wire.Message_RequestPhoneNumberMessage{}}, {EditedMessage: &wire.Message_FutureProofMessage{}},
		{PinInChatMessage: &wire.Message_PinInChatMessage{}}, {EncCommentMessage: &wire.Message_EncCommentMessage{}},
		{NewsletterAdminInviteMessage: &wire.Message_NewsletterAdminInviteMessage{}}, {NewsletterFollowerInviteMessageV2: &wire.Message_NewsletterFollowerInviteMessage{}},
		{MessageHistoryNotice: &wire.Message_MessageHistoryNotice{}}, {AlbumMessage: &wire.Message_AlbumMessage{}},
		{RichResponseMessage: &wire.AIRichResponseMessage{}}, {ExtendedTextMessage: &wire.Message_ExtendedTextMessage{}},
	}
	for i, m := range textual {
		if got := message.TypeOf(m); got != message.TypeText {
			t.Errorf("textual kind %d = %q", i, got)
		}
	}
}

func TestSentByUsKeepsTheContextOutside(t *testing.T) {
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	m := &wire.Message{Conversation: new("hi"), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: []byte{1}}}
	wrapped := message.SentByUs(bob, m)
	if wrapped.GetDeviceSentMessage().GetDestinationJid() != bob.String() || wrapped.GetMessageContextInfo() == nil ||
		wrapped.GetDeviceSentMessage().GetMessage().GetMessageContextInfo() != nil || wrapped.GetDeviceSentMessage().GetMessage().GetConversation() != "hi" {
		t.Fatalf("SentByUs() = %v", wrapped)
	}
	if m.GetMessageContextInfo() == nil {
		t.Fatal("SentByUs changed the caller's message")
	}
}

func TestOutgoingLayout(t *testing.T) {
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	companion := node.JID{User: bob.User, Device: 3, Server: bob.Server}
	msg := signal.Ciphertext{Type: signal.TypeMessage, Bytes: []byte{1}}
	pkmsg := signal.Ciphertext{Type: signal.TypePreKeyMessage, Bytes: []byte{2}}
	hi := &wire.Message{Conversation: new("hi")}
	tags := func(n node.Node) string {
		out := make([]string, 0, len(n.Children))
		for _, c := range n.Children {
			out = append(out, c.Tag)
		}
		return strings.Join(out, ",")
	}
	direct := message.Outgoing("3EB0AA", bob, hi, []message.Part{{Device: bob, Ciphertext: pkmsg}}, []byte("id"))
	enc, _ := direct.Child("enc")
	if tags(direct) != "enc,device-identity" || !bytes.Equal(enc.Bytes, []byte{2}) || enc.Attr("type").String() != "pkmsg" || enc.Attr("v").String() != "2" {
		t.Fatalf("one primary device: %s", direct)
	}
	for i, a := range []string{"id=3EB0AA", "to=40722222222@s.whatsapp.net", "type=text"} {
		if got := direct.Attrs[i].Key + "=" + direct.Attrs[i].Value.String(); got != a {
			t.Fatalf("attribute %d = %s, want %s", i, got, a)
		}
	}
	one := message.Outgoing("3EB0AA", bob, hi, []message.Part{{Device: companion, Ciphertext: msg}}, []byte("id"))
	if tags(one) != "participants" {
		t.Fatalf("one companion device, no prekey message: %s", one)
	}
	both := message.Outgoing("3EB0AA", bob, hi, []message.Part{{Device: bob, Ciphertext: msg}, {Device: companion, Ciphertext: pkmsg}}, []byte("id"))
	participants, _ := both.Child("participants")
	if tags(both) != "participants,device-identity" || len(participants.Children) != 2 {
		t.Fatalf("two devices: %s", both)
	}
	second, _ := participants.Children[1].Attr("jid").JID()
	if inner, _ := participants.Children[1].Child("enc"); second != companion || inner.Attr("type").String() != "pkmsg" {
		t.Fatalf("second target = %s", participants.Children[1])
	}
	if tags(message.Outgoing("3EB0AA", bob, hi, []message.Part{{Device: companion, Ciphertext: pkmsg}}, nil)) != "participants" {
		t.Fatal("device identity added although we have none")
	}
}

func TestNewID(t *testing.T) {
	self := node.JID{User: "111111111111111", Server: node.ServerLID}
	a, err := message.NewID(time.Unix(1, 0), self, bytes.NewReader(make([]byte, 16)))
	if err != nil || len(a) != 22 || !strings.HasPrefix(a, "3EB0") || strings.ToUpper(a) != a {
		t.Fatalf("NewID() = %q, %v", a, err)
	}
	for _, other := range []struct {
		at   time.Time
		self node.JID
		seed byte
	}{{time.Unix(2, 0), self, 0}, {time.Unix(1, 0), node.JID{User: "2", Server: node.ServerLID}, 0}, {time.Unix(1, 0), self, 1}} {
		b, err := message.NewID(other.at, other.self, bytes.NewReader(bytes.Repeat([]byte{other.seed}, 16)))
		if err != nil || b == a {
			t.Fatalf("NewID(%v) = %q, same as %q", other, b, a)
		}
	}
	if _, err := message.NewID(time.Unix(1, 0), self, bytes.NewReader(make([]byte, 15))); err == nil {
		t.Fatal("NewID succeeded with 15 random bytes")
	}
}

func TestOutgoingActionsAreMarkedLikeWhatsAppWeb(t *testing.T) {
	t.Parallel()
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	key := &wire.MessageKey{RemoteJid: new(bob.String()), FromMe: new(true), Id: new("3EB0TARGET")}
	protocol := func(kind wire.Message_ProtocolMessage_Type) *wire.Message {
		return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key, Type: kind.Enum()}}
	}
	reaction := func(emoji string) *wire.Message {
		return &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Key: key, Text: new(emoji)}}
	}
	ephemeral := func(m *wire.Message) *wire.Message {
		return &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: m}}
	}
	vote := &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{PollCreationMessageKey: key, Vote: &wire.Message_PollEncValue{EncPayload: []byte{1}, EncIv: []byte{2}}}}
	tests := []struct {
		name     string
		msg      *wire.Message
		wantType string
		wantEdit string
		wantPoll string
	}{
		{name: "text", msg: &wire.Message{Conversation: new("hi")}, wantType: "text"},
		{name: "delete for everyone", msg: protocol(wire.Message_ProtocolMessage_REVOKE), wantType: "text", wantEdit: "7"},
		{name: "edit", msg: protocol(wire.Message_ProtocolMessage_MESSAGE_EDIT), wantType: "text", wantEdit: "1"},
		{name: "other protocol message", msg: protocol(wire.Message_ProtocolMessage_EPHEMERAL_SETTING), wantType: "text"},
		{name: "protocol message without a type", msg: &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key}}, wantType: "text"},
		{name: "reaction", msg: reaction("👍"), wantType: "reaction"},
		{name: "reaction taken back", msg: reaction(""), wantType: "reaction", wantEdit: "7"},
		{name: "reaction without text", msg: &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Key: key}}, wantType: "reaction"},
		{name: "wrapped delete", msg: ephemeral(protocol(wire.Message_ProtocolMessage_REVOKE)), wantType: "text", wantEdit: "7"},
		{name: "poll", msg: &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?")}}, wantType: "poll", wantPoll: "creation"},
		{name: "wrapped poll", msg: ephemeral(&wire.Message{PollCreationMessage: &wire.Message_PollCreationMessage{Name: new("Lunch?")}}), wantType: "poll", wantPoll: "creation"},
		{name: "vote", msg: vote, wantType: "poll", wantPoll: "vote"},
		{name: "poll update without a vote", msg: &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{PollCreationMessageKey: key}}, wantType: "poll"},
	}
	part := []message.Part{{Device: node.JID{User: bob.User, Device: 2, Server: bob.Server}, Ciphertext: signal.Ciphertext{Type: signal.TypePreKeyMessage, Bytes: []byte{1}}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			edit := ""
			if got := message.EditOf(tt.msg); got != message.EditNone {
				edit = strconv.Itoa(int(got))
			}
			if edit != tt.wantEdit {
				t.Fatalf("EditOf() = %q, want %q", edit, tt.wantEdit)
			}
			if got := message.PollType(tt.msg); got != tt.wantPoll {
				t.Fatalf("PollType() = %q, want %q", got, tt.wantPoll)
			}
			wantAttrs := []string{"id=3EB0AA", "to=" + bob.String(), "type=" + tt.wantType}
			if tt.wantEdit != "" {
				wantAttrs = append(wantAttrs, "edit="+tt.wantEdit)
			}
			wantChildren := []string{"participants", "device-identity"}
			if tt.wantPoll != "" {
				wantChildren = append(wantChildren, "meta")
			}
			direct := message.Outgoing("3EB0AA", bob, tt.msg, part, []byte("id"))
			checkStanza(t, direct, wantAttrs, wantChildren, tt.wantPoll)
			group := message.OutgoingGroup("3EB0AA", family, tt.msg, "lid", part, []byte{9}, []byte("id"))
			wantAttrs[1] = "to=" + family.String()
			checkStanza(t, group, append(wantAttrs, "addressing_mode=lid"), slices.Insert(wantChildren, 1, "enc"), tt.wantPoll)
		})
	}
}

func TestGroupMessagesWithoutPairwiseParts(t *testing.T) {
	t.Parallel()
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	hi := &wire.Message{Conversation: new("hi")}
	out := message.OutgoingGroup("3EB0AA", family, hi, "", nil, []byte{9}, []byte("id"))
	checkStanza(t, out, []string{"id=3EB0AA", "to=" + family.String(), "type=text"}, []string{"enc"}, "")
	if enc, _ := out.Child("enc"); enc.Attr("type").String() != "skmsg" || !bytes.Equal(enc.Bytes, []byte{9}) {
		t.Fatalf("sender key message = %s", enc)
	}
}

func checkStanza(t *testing.T, n node.Node, attrs, children []string, poll string) {
	t.Helper()
	got := make([]string, 0, len(n.Attrs))
	for _, a := range n.Attrs {
		got = append(got, a.Key+"="+a.Value.String())
	}
	if !slices.Equal(got, attrs) {
		t.Fatalf("attributes %q, want %q", got, attrs)
	}
	tags := make([]string, 0, len(n.Children))
	for _, c := range n.Children {
		tags = append(tags, c.Tag)
	}
	if !slices.Equal(tags, children) {
		t.Fatalf("children %q, want %q", tags, children)
	}
	if m, ok := n.Child("meta"); poll != "" && (!ok || len(m.Attrs) != 1 || m.Attr("polltype").String() != poll) {
		t.Fatalf("meta = %s", m)
	}
}
