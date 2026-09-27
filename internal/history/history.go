package history

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const MaxInflated = 256 << 20

var (
	ErrNoPayload = errors.New("history: notification carries no payload")
	ErrTooLarge  = errors.New("history: inflated payload too large")
	ErrMalformed = errors.New("history: malformed payload")
)

type Chunk struct {
	Type      wire.HistorySync_HistorySyncType
	Order     uint32
	Progress  uint32
	Chats     []Chat
	Messages  []Message
	PushNames map[node.JID]string
	Contacts  []Contact
	LIDs      map[node.JID]node.JID
}

type Chat struct {
	JID         node.JID
	Name        string
	Unread      uint32
	LastMessage time.Time
	Archived    bool
	Pinned      bool
	ReadOnly    bool
	MutedUntil  time.Time
	Token       privacy.Token
}

type Message struct {
	ID        string
	Chat      node.JID
	Author    node.JID
	FromMe    bool
	Time      time.Time
	PushName  string
	Message   *wire.Message
	Reactions []Reaction
	Votes     []Vote
}

type Reaction struct {
	By    node.JID
	Emoji string
	Time  time.Time
}

type Vote struct {
	By      node.JID
	Options []string
	Time    time.Time
}

type Contact struct {
	JID       node.JID
	LID       node.JID
	FullName  string
	FirstName string
}

func Payload(n *wire.Message_HistorySyncNotification) ([]byte, media.Reference, error) {
	inlinable := []wire.Message_HistorySyncType{wire.Message_INITIAL_BOOTSTRAP, wire.Message_INITIAL_STATUS_V3, wire.Message_PUSH_NAME, wire.Message_ON_DEMAND}
	if inline := n.GetInitialHistBootstrapInlinePayload(); len(inline) > 0 && slices.Contains(inlinable, n.GetSyncType()) {
		return inline, media.Reference{}, nil
	}
	if n.GetDirectPath() == "" || len(n.GetMediaKey()) != media.KeySize {
		return nil, media.Reference{}, ErrNoPayload
	}
	return nil, media.Reference{
		Type: media.History, DirectPath: n.GetDirectPath(), MediaKey: n.GetMediaKey(),
		FileSHA256: n.GetFileSha256(), FileEncSHA256: n.GetFileEncSha256(), Length: n.GetFileLength(),
	}, nil
}

func Inflate(data []byte) ([]byte, error) {
	var (
		r   io.Reader
		err error
	)
	switch {
	case len(data) >= 3 && data[0] == 0x1f && data[1] == 0x8b && data[2] == 8:
		var gz *gzip.Reader
		if gz, err = gzip.NewReader(bytes.NewReader(data)); err == nil {
			gz.Multistream(false)
			r = gz
		}
	case len(data) >= 2 && data[0]&15 == 8 && data[0]>>4 <= 7 && (uint(data[0])<<8|uint(data[1]))%31 == 0:
		r, err = zlib.NewReader(bytes.NewReader(data))
	default:
		r = flate.NewReader(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	out, err := io.ReadAll(io.LimitReader(r, MaxInflated+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if len(out) > MaxInflated {
		return nil, ErrTooLarge
	}
	return out, nil
}

func Parse(data []byte, self node.JID) (Chunk, error) {
	var sync wire.HistorySync
	if err := proto.Unmarshal(data, &sync); err != nil {
		return Chunk{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	chunk := Chunk{
		Type: sync.GetSyncType(), Order: sync.GetChunkOrder(), Progress: sync.GetProgress(),
		PushNames: map[node.JID]string{}, LIDs: map[node.JID]node.JID{},
	}
	chunk.Chats, chunk.Messages = conversations(sync.GetConversations(), self)
	for _, p := range sync.GetPushnames() {
		if jid, err := node.ParseJID(p.GetId()); err == nil && p.GetPushname() != "" {
			chunk.PushNames[jid] = p.GetPushname()
		}
	}
	for _, m := range sync.GetPhoneNumberToLidMappings() {
		pn, pnErr := node.ParseJID(m.GetPnJid())
		lid, lidErr := node.ParseJID(m.GetLidJid())
		if pnErr == nil && lidErr == nil && pn.Server == node.ServerUser && lid.Server == node.ServerLID {
			chunk.LIDs[lid] = pn
		}
	}
	chunk.Contacts = contacts(sync.GetInlineContacts())
	return chunk, nil
}

func conversations(convs []*wire.Conversation, self node.JID) ([]Chat, []Message) {
	var (
		chats    []Chat
		messages []Message
	)
	for _, c := range convs {
		jid, err := node.ParseJID(c.GetId())
		if err != nil {
			continue
		}
		chats = append(chats, chatOf(jid, c))
		for _, m := range c.GetMessages() {
			if parsed, ok := messageOf(jid, self, m.GetMessage()); ok {
				messages = append(messages, parsed)
			}
		}
	}
	return chats, messages
}

func contacts(inline []*wire.InlineContact) []Contact {
	var out []Contact
	for _, c := range inline {
		contact := Contact{FullName: c.GetFullName(), FirstName: c.GetFirstName()}
		contact.JID, _ = node.ParseJID(c.GetPnJid())
		contact.LID, _ = node.ParseJID(c.GetLidJid())
		if contact.JID.Server != "" || contact.LID.Server != "" {
			out = append(out, contact)
		}
	}
	return out
}

func chatOf(jid node.JID, c *wire.Conversation) Chat {
	chat := Chat{
		JID: jid, Name: c.GetName(), Unread: c.GetUnreadCount(), Archived: c.GetArchived(), Pinned: c.GetPinned() > 0, ReadOnly: c.GetReadOnly(),
	}
	if chat.Name == "" {
		chat.Name = c.GetDisplayName()
	}
	if last := c.GetLastMsgTimestamp(); last > 0 {
		chat.LastMessage = unix(last)
	} else if last := c.GetConversationTimestamp(); last > 0 {
		chat.LastMessage = unix(last)
	}
	if muted := c.GetMuteEndTime(); muted > 0 {
		chat.MutedUntil = unix(muted)
	}
	chat.Token = privacy.Token{Contact: jid, Theirs: c.GetTcToken()}
	if given := c.GetTcTokenTimestamp(); given > 0 && len(chat.Token.Theirs) > 0 {
		chat.Token.Given = unix(given)
	}
	if ours := c.GetTcTokenSenderTimestamp(); ours > 0 {
		chat.Token.Ours = unix(ours)
	}
	return chat
}

func messageOf(chat, self node.JID, info *wire.MessageInfo) (Message, bool) {
	key := info.GetKey()
	if key.GetId() == "" || !hasContent(info.GetMessage()) || media.Unwrap(info.GetMessage()).GetPollUpdateMessage() != nil {
		return Message{}, false
	}
	m := Message{
		ID: key.GetId(), Chat: chat, FromMe: key.GetFromMe(), Time: unix(info.GetMessageTimestamp()),
		PushName: info.GetPushName(), Message: info.GetMessage(),
	}
	switch {
	case m.FromMe:
		m.Author = self
	case chat.Server == node.ServerGroup || chat.Server == node.ServerBroadcast:
		participant := key.GetParticipant()
		if participant == "" {
			participant = info.GetParticipant()
		}
		author, err := node.ParseJID(participant)
		if err != nil {
			return Message{}, false
		}
		m.Author = author
	default:
		m.Author = chat
	}
	for _, r := range info.GetReactions() {
		if r.GetText() == "" {
			continue
		}
		by, ok := reactor(chat, self, r.GetKey())
		if ok {
			m.Reactions = append(m.Reactions, Reaction{By: by, Emoji: r.GetText(), Time: time.UnixMilli(max(r.GetSenderTimestampMs(), 0))})
		}
	}
	if poll := message.PollOf(media.Unwrap(m.Message)); poll != nil {
		m.Votes = votes(chat, self, info, poll)
	}
	if secret := info.GetMessageSecret(); len(secret) > 0 && len(m.Message.GetMessageContextInfo().GetMessageSecret()) == 0 {
		if m.Message.MessageContextInfo == nil {
			m.Message.MessageContextInfo = &wire.MessageContextInfo{}
		}
		m.Message.MessageContextInfo.MessageSecret = secret
	}
	return m, true
}

func votes(chat, self node.JID, info *wire.MessageInfo, poll *wire.Message_PollCreationMessage) []Vote {
	var out []Vote
	add := func(key *wire.MessageKey, vote *wire.Message_PollVoteMessage, ms int64) {
		by, ok := reactor(chat, self, key)
		if !ok || vote == nil {
			return
		}
		if chosen, err := message.Chosen(poll, vote); err == nil {
			out = append(out, Vote{By: by, Options: chosen, Time: time.UnixMilli(max(ms, 0))})
		}
	}
	if addOns := info.GetMessageAddOns(); len(addOns) > 0 {
		for _, a := range addOns {
			if a.GetMessageAddOnType() == wire.MessageAddOn_POLL_UPDATE {
				add(a.GetMessageAddOnKey(), a.GetLegacyMessage().GetPollVote(), a.GetSenderTimestampMs())
			}
		}
		return out
	}
	for _, u := range info.GetPollUpdates() {
		add(u.GetPollUpdateMessageKey(), u.GetVote(), u.GetSenderTimestampMs())
	}
	return out
}

func reactor(chat, self node.JID, key *wire.MessageKey) (node.JID, bool) {
	switch {
	case key.GetFromMe():
		return self, true
	case key.GetParticipant() != "":
		by, err := node.ParseJID(key.GetParticipant())
		return by, err == nil
	case chat.Server == node.ServerGroup || chat.Server == node.ServerBroadcast:
		return node.JID{}, false
	default:
		return chat, true
	}
}

func hasContent(m *wire.Message) bool {
	if m == nil {
		return false
	}
	rest := proto.CloneOf(m)
	rest.SenderKeyDistributionMessage, rest.MessageContextInfo, rest.ProtocolMessage = nil, nil, nil
	return proto.Size(rest) > 0
}

func unix(seconds uint64) time.Time {
	return time.Unix(int64(min(seconds, uint64(1)<<62)), 0)
}
