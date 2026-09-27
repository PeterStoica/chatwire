package history_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"crypto/rand"
	"errors"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestPayloadIsInlineOnlyForTheTypesWhatsAppWebInlines(t *testing.T) {
	t.Parallel()
	mediaKey := bytes.Repeat([]byte{1}, media.KeySize)
	inline := []byte("inline")
	tests := []struct {
		name     string
		syncType wire.Message_HistorySyncType
		inlined  bool
	}{
		{name: "initial bootstrap", syncType: wire.Message_INITIAL_BOOTSTRAP, inlined: true},
		{name: "initial status", syncType: wire.Message_INITIAL_STATUS_V3, inlined: true},
		{name: "push names", syncType: wire.Message_PUSH_NAME, inlined: true},
		{name: "on demand", syncType: wire.Message_ON_DEMAND, inlined: true},
		{name: "recent", syncType: wire.Message_RECENT},
		{name: "full", syncType: wire.Message_FULL},
		{name: "non blocking", syncType: wire.Message_NON_BLOCKING_DATA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := &wire.Message_HistorySyncNotification{
				SyncType: tt.syncType.Enum(), InitialHistBootstrapInlinePayload: inline, DirectPath: new("/v/hist.enc"),
				MediaKey: mediaKey, FileSha256: []byte{1}, FileEncSha256: []byte{2}, FileLength: new(uint64(99)),
			}
			data, ref, err := history.Payload(n)
			if err != nil {
				t.Fatal(err)
			}
			if tt.inlined {
				if !bytes.Equal(data, inline) || ref.DirectPath != "" {
					t.Fatalf("Payload() = %q, %+v; want the inline payload", data, ref)
				}
				return
			}
			want := media.Reference{Type: media.History, DirectPath: "/v/hist.enc", MediaKey: mediaKey, FileSHA256: []byte{1}, FileEncSHA256: []byte{2}, Length: 99}
			if data != nil || ref.Type != want.Type || ref.DirectPath != want.DirectPath || !bytes.Equal(ref.MediaKey, want.MediaKey) ||
				!bytes.Equal(ref.FileSHA256, want.FileSHA256) || !bytes.Equal(ref.FileEncSHA256, want.FileEncSHA256) || ref.Length != want.Length {
				t.Fatalf("Payload() = %q, %+v; want %+v", data, ref, want)
			}
		})
	}
}

func TestPayloadFallsBackToTheFileAndNeedsOne(t *testing.T) {
	t.Parallel()
	mediaKey := bytes.Repeat([]byte{1}, media.KeySize)
	bootstrap := wire.Message_INITIAL_BOOTSTRAP.Enum()
	if _, ref, err := history.Payload(&wire.Message_HistorySyncNotification{SyncType: bootstrap, InitialHistBootstrapInlinePayload: []byte{}, DirectPath: new("/v/x"), MediaKey: mediaKey}); err != nil || ref.DirectPath != "/v/x" {
		t.Fatalf("an empty inline payload must fall back to the file: %+v, %v", ref, err)
	}
	for _, n := range []*wire.Message_HistorySyncNotification{
		{SyncType: bootstrap},
		{SyncType: wire.Message_RECENT.Enum(), InitialHistBootstrapInlinePayload: []byte("x"), MediaKey: mediaKey},
		{SyncType: bootstrap, DirectPath: new("/v/x"), MediaKey: mediaKey[:31]},
		{SyncType: bootstrap, DirectPath: new("/v/x"), MediaKey: append(mediaKey, 0)},
	} {
		if _, _, err := history.Payload(n); !errors.Is(err, history.ErrNoPayload) {
			t.Errorf("Payload(%v) = %v, want %v", n, err, history.ErrNoPayload)
		}
	}
}

func compress(t *testing.T, format string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	var w interface {
		Write([]byte) (int, error)
		Close() error
	}
	var err error
	switch format {
	case "gzip":
		w = gzip.NewWriter(&buf)
	case "zlib":
		w = zlib.NewWriter(&buf)
	default:
		w, err = flate.NewWriter(&buf, flate.BestCompression)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInflatingEveryFormatWhatsAppWebAccepts(t *testing.T) {
	t.Parallel()
	random := make([]byte, 10000)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"gzip", "zlib", "raw"} {
		for _, data := range [][]byte{{}, []byte("history"), bytes.Repeat([]byte("abc"), 50000), random} {
			got, err := history.Inflate(compress(t, format, data))
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("%s of %d bytes: got %d bytes, %v", format, len(data), len(got), err)
			}
		}
	}
	first, second := compress(t, "gzip", []byte("first")), compress(t, "gzip", []byte("second"))
	if got, err := history.Inflate(append(first, second...)); err != nil || string(got) != "first" {
		t.Fatalf("only the first gzip member counts, like fflate: %q, %v", got, err)
	}
}

func TestInflateRefusesDamage(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte("history "), 1000)
	corrupt := func(b []byte, at int) []byte {
		c := bytes.Clone(b)
		c[at] ^= 0xff
		return c
	}
	gz, zl, raw := compress(t, "gzip", data), compress(t, "zlib", data), compress(t, "raw", data)
	tests := []struct {
		name string
		in   []byte
	}{
		{name: "empty", in: nil},
		{name: "gzip crc", in: corrupt(gz, len(gz)-6)},
		{name: "gzip truncated", in: gz[:len(gz)/2]},
		{name: "zlib adler", in: corrupt(zl, len(zl)-1)},
		{name: "zlib truncated", in: zl[:len(zl)/2]},
		{name: "zlib preset dictionary", in: append([]byte{0x78, 0xbb}, zl[2:]...)},
		{name: "raw truncated", in: raw[:len(raw)/2]},
		{name: "gzip magic then garbage", in: []byte{0x1f, 0x8b, 8, 0xff}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := history.Inflate(tt.in); !errors.Is(err, history.ErrMalformed) {
				t.Fatalf("Inflate() = %d bytes, %v", len(got), err)
			}
		})
	}
}

func TestInflateStopsAtTheCap(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	zeros := make([]byte, 1<<20)
	for range history.MaxInflated >> 20 {
		if _, err := w.Write(zeros); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Inflate(buf.Bytes()); !errors.Is(err, history.ErrTooLarge) {
		t.Fatalf("one byte over the cap: %v", err)
	}
	var atCap bytes.Buffer
	cw, err := flate.NewWriter(&atCap, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	for range history.MaxInflated >> 20 {
		if _, err := cw.Write(zeros); err != nil {
			t.Fatal(err)
		}
	}
	if err := cw.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := history.Inflate(atCap.Bytes()); err != nil || len(got) != history.MaxInflated {
		t.Fatalf("exactly the cap: %d bytes, %v", len(got), err)
	}
}

func webMessage(remote string, fromMe bool, id, participant string, at uint64, m *wire.Message) *wire.HistorySyncMsg {
	key := &wire.MessageKey{RemoteJid: new(remote), FromMe: new(fromMe), Id: new(id)}
	if participant != "" {
		key.Participant = new(participant)
	}
	return &wire.HistorySyncMsg{Message: &wire.MessageInfo{Key: key, Message: m, MessageTimestamp: new(at), PushName: new("Bob")}}
}

func TestParsingAHistoryChunk(t *testing.T) {
	t.Parallel()
	self := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	lidChat := node.JID{User: "88123456789012", Server: node.ServerLID}
	text := func(s string) *wire.Message { return &wire.Message{Conversation: new(s)} }
	fromInfo := webMessage(family.String(), false, "3EB0G2", "", 1790000102, text("participant on the info"))
	fromInfo.Message.Participant = new("40733333333@s.whatsapp.net")
	sync := &wire.HistorySync{
		SyncType: wire.HistorySync_INITIAL_BOOTSTRAP.Enum(), ChunkOrder: new(uint32(2)), Progress: new(uint32(40)),
		Conversations: []*wire.Conversation{
			{
				Id: new(bob.String()), DisplayName: new("Bob Contact"), UnreadCount: new(uint32(3)), LastMsgTimestamp: new(uint64(1790000010)),
				Archived: new(true), Pinned: new(uint32(1790000000)), MuteEndTime: new(uint64(1790009999)),
				TcToken: []byte{5, 5}, TcTokenTimestamp: new(uint64(1790000300)), TcTokenSenderTimestamp: new(uint64(1790000400)),
				EphemeralExpiration: new(uint32(604800)), EphemeralSettingTimestamp: new(int64(1789999000)),
				Messages: []*wire.HistorySyncMsg{
					webMessage(bob.String(), false, "3EB0B1", "", 1790000001, text("hi from bob")),
					webMessage(bob.String(), true, "3EB0B2", "", 1790000002, text("hi from me")),
					webMessage(bob.String(), false, "", "", 1790000003, text("no id")),
					webMessage(bob.String(), false, "3EB0B4", "", 1790000004, nil),
					webMessage(bob.String(), false, "3EB0B5", "", 1790000005, &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Type: wire.Message_ProtocolMessage_REVOKE.Enum()}}),
					webMessage(bob.String(), false, "3EB0B6", "", 1790000006, &wire.Message{SenderKeyDistributionMessage: &wire.Message_SenderKeyDistributionMessage{}, MessageContextInfo: &wire.MessageContextInfo{}}),
				},
			},
			{
				Id: new(family.String()), Name: new("Family"), DisplayName: new("ignored"), ConversationTimestamp: new(uint64(1790000200)), ReadOnly: new(true),
				Messages: []*wire.HistorySyncMsg{
					webMessage(family.String(), false, "3EB0G1", bob.String(), 1790000101, text("participant on the key")),
					fromInfo,
					webMessage(family.String(), false, "3EB0G3", "", 1790000103, text("no participant")),
					webMessage(family.String(), true, "3EB0G4", "", 1790000104, text("mine in a group")),
				},
			},
			{Id: new(lidChat.String())},
			{Id: new("not a jid")},
		},
		Pushnames: []*wire.Pushname{
			{Id: new(bob.String()), Pushname: new("Bobby")},
			{Id: new("bad"), Pushname: new("x")},
			{Id: new("40744444444@s.whatsapp.net"), Pushname: new("")},
		},
		PhoneNumberToLidMappings: []*wire.PhoneNumberToLIDMapping{
			{PnJid: new(bob.String()), LidJid: new(lidChat.String())},
			{PnJid: new(lidChat.String()), LidJid: new(bob.String())},
			{PnJid: new("bad"), LidJid: new(lidChat.String())},
		},
		InlineContacts: []*wire.InlineContact{
			{PnJid: new(bob.String()), LidJid: new(lidChat.String()), FullName: new("Bob Builder"), FirstName: new("Bob")},
			{FullName: new("nobody")},
		},
	}
	data, err := proto.Marshal(sync)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := history.Parse(data, self)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.Type != wire.HistorySync_INITIAL_BOOTSTRAP || chunk.Order != 2 || chunk.Progress != 40 {
		t.Fatalf("chunk header = %v %d %d", chunk.Type, chunk.Order, chunk.Progress)
	}
	wantChats := []history.Chat{
		{JID: bob, Name: "Bob Contact", Unread: 3, LastMessage: time.Unix(1790000010, 0), Archived: true, Pinned: true, MutedUntil: time.Unix(1790009999, 0),
			Token: privacy.Token{Contact: bob, Theirs: []byte{5, 5}, Given: time.Unix(1790000300, 0), Ours: time.Unix(1790000400, 0)},
			Timer: history.Timer{Seconds: 604800, Set: time.Unix(1789999000, 0)}},
		{JID: family, Name: "Family", LastMessage: time.Unix(1790000200, 0), ReadOnly: true, Token: privacy.Token{Contact: family}},
		{JID: lidChat, Token: privacy.Token{Contact: lidChat}},
	}
	if len(chunk.Chats) != len(wantChats) {
		t.Fatalf("chats = %+v", chunk.Chats)
	}
	for i, want := range wantChats {
		if got := chunk.Chats[i]; !sameChat(got, want) {
			t.Errorf("chat %d = %+v, want %+v", i, got, want)
		}
	}
	wantMessages := []struct {
		id     string
		chat   node.JID
		author node.JID
		fromMe bool
		at     int64
	}{
		{"3EB0B1", bob, bob, false, 1790000001},
		{"3EB0B2", bob, self, true, 1790000002},
		{"3EB0G1", family, bob, false, 1790000101},
		{"3EB0G2", family, node.JID{User: "40733333333", Server: node.ServerUser}, false, 1790000102},
		{"3EB0G4", family, self, true, 1790000104},
	}
	if len(chunk.Messages) != len(wantMessages) {
		t.Fatalf("messages = %+v", chunk.Messages)
	}
	for i, want := range wantMessages {
		got := chunk.Messages[i]
		if got.ID != want.id || got.Chat != want.chat || got.Author != want.author || got.FromMe != want.fromMe || !got.Time.Equal(time.Unix(want.at, 0)) || got.PushName != "Bob" || got.Message == nil {
			t.Errorf("message %d = %+v, want %+v", i, got, want)
		}
	}
	if len(chunk.PushNames) != 1 || chunk.PushNames[bob] != "Bobby" {
		t.Fatalf("push names = %v", chunk.PushNames)
	}
	if len(chunk.LIDs) != 1 || chunk.LIDs[lidChat] != bob {
		t.Fatalf("lids = %v", chunk.LIDs)
	}
	if len(chunk.Contacts) != 1 || chunk.Contacts[0] != (history.Contact{JID: bob, LID: lidChat, FullName: "Bob Builder", FirstName: "Bob"}) {
		t.Fatalf("contacts = %+v", chunk.Contacts)
	}
}

func TestParseRefusesGarbageAndSurvivesExtremeTimes(t *testing.T) {
	t.Parallel()
	if _, err := history.Parse([]byte{0xff, 0xff, 0xff}, node.JID{}); !errors.Is(err, history.ErrMalformed) {
		t.Fatalf("Parse(garbage) = %v", err)
	}
	sync := &wire.HistorySync{SyncType: wire.HistorySync_RECENT.Enum(), Conversations: []*wire.Conversation{{
		Id: new("40722222222@s.whatsapp.net"), LastMsgTimestamp: new(uint64(1) << 63),
		Messages: []*wire.HistorySyncMsg{webMessage("40722222222@s.whatsapp.net", false, "3EB0X", "", ^uint64(0), &wire.Message{Conversation: new("far future")})},
	}}}
	data, err := proto.Marshal(sync)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := history.Parse(data, node.JID{})
	if err != nil || len(chunk.Messages) != 1 || chunk.Messages[0].Time.Unix() != 1<<62 || chunk.Chats[0].LastMessage.Unix() != 1<<62 {
		t.Fatalf("Parse() = %+v, %v", chunk, err)
	}
	if _, err := history.Parse(nil, node.JID{}); !errors.Is(err, history.ErrMalformed) {
		t.Fatalf("a chunk without its required sync type: %v", err)
	}
	minimal, err := proto.Marshal(&wire.HistorySync{SyncType: wire.HistorySync_PUSH_NAME.Enum()})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := history.Parse(minimal, node.JID{})
	if err != nil || empty.Type != wire.HistorySync_PUSH_NAME || len(empty.Chats) != 0 || empty.PushNames == nil || empty.LIDs == nil {
		t.Fatalf("Parse(minimal) = %+v, %v", empty, err)
	}
}

func TestReactionsInHistory(t *testing.T) {
	t.Parallel()
	self := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	reaction := func(fromMe bool, participant, emoji string, ms int64) *wire.Reaction {
		key := &wire.MessageKey{FromMe: new(fromMe), Id: new("R")}
		if participant != "" {
			key.Participant = new(participant)
		}
		return &wire.Reaction{Key: key, Text: new(emoji), SenderTimestampMs: new(ms)}
	}
	withReactions := func(m *wire.HistorySyncMsg, r ...*wire.Reaction) *wire.HistorySyncMsg {
		m.Message.Reactions = r
		return m
	}
	sync := &wire.HistorySync{SyncType: wire.HistorySync_RECENT.Enum(), Conversations: []*wire.Conversation{
		{Id: new(bob.String()), Messages: []*wire.HistorySyncMsg{withReactions(webMessage(bob.String(), true, "M1", "", 1790000000, &wire.Message{Conversation: new("hi")}),
			reaction(false, "", "👍", 1790000001000), reaction(true, "", "❤️", 1790000002000), reaction(false, "", "", 1790000003000))}},
		{Id: new(family.String()), Messages: []*wire.HistorySyncMsg{withReactions(webMessage(family.String(), false, "G1", bob.String(), 1790000000, &wire.Message{Conversation: new("dinner")}),
			reaction(false, carol.String(), "😂", 1790000004000), reaction(false, "", "👀", 1790000005000), reaction(false, "not a jid", "🙃", 1))}},
	}}
	data, err := proto.Marshal(sync)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := history.Parse(data, self)
	if err != nil || len(chunk.Messages) != 2 {
		t.Fatalf("Parse() = %+v, %v", chunk.Messages, err)
	}
	want := [][]history.Reaction{
		{{By: bob, Emoji: "👍", Time: time.UnixMilli(1790000001000)}, {By: self, Emoji: "❤️", Time: time.UnixMilli(1790000002000)}},
		{{By: carol, Emoji: "😂", Time: time.UnixMilli(1790000004000)}},
	}
	for i, m := range chunk.Messages {
		if len(m.Reactions) != len(want[i]) {
			t.Fatalf("message %s reactions = %+v", m.ID, m.Reactions)
		}
		for j, r := range m.Reactions {
			if r.By != want[i][j].By || r.Emoji != want[i][j].Emoji || !r.Time.Equal(want[i][j].Time) {
				t.Fatalf("message %s reaction %d = %+v, want %+v", m.ID, j, r, want[i][j])
			}
		}
	}
}

func TestPollsInHistory(t *testing.T) {
	t.Parallel()
	self := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	poll := &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), SelectableOptionsCount: new(uint32(1)), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}}}}
	key := func(fromMe bool, participant string) *wire.MessageKey {
		k := &wire.MessageKey{FromMe: new(fromMe), Id: new("V")}
		if participant != "" {
			k.Participant = new(participant)
		}
		return k
	}
	picked := func(names ...string) *wire.Message_PollVoteMessage {
		vote := &wire.Message_PollVoteMessage{}
		for _, name := range names {
			vote.SelectedOptions = append(vote.SelectedOptions, message.OptionHash(name))
		}
		return vote
	}
	updates := webMessage(family.String(), false, "P1", bob.String(), 1790000000, poll)
	updates.Message.MessageSecret = bytes.Repeat([]byte{5}, 32)
	updates.Message.PollUpdates = []*wire.PollUpdate{
		{PollUpdateMessageKey: key(false, carol.String()), Vote: picked("Sushi"), SenderTimestampMs: new(int64(1790000001000))},
		{PollUpdateMessageKey: key(true, ""), Vote: picked("Pizza"), SenderTimestampMs: new(int64(1790000002000))},
		{PollUpdateMessageKey: key(false, bob.String()), Vote: picked("Pizza", "Sushi"), SenderTimestampMs: new(int64(1790000003000))},
		{PollUpdateMessageKey: key(false, bob.String()), Vote: picked("Soup"), SenderTimestampMs: new(int64(1790000004000))},
		{PollUpdateMessageKey: key(false, ""), Vote: picked("Pizza"), SenderTimestampMs: new(int64(1790000005000))},
		{PollUpdateMessageKey: key(false, bob.String()), SenderTimestampMs: new(int64(1790000006000))},
		{PollUpdateMessageKey: key(false, bob.String()), Vote: picked(), SenderTimestampMs: new(int64(1790000007000))},
	}
	addOns := webMessage(bob.String(), true, "P2", "", 1790000000, poll)
	addOns.Message.MessageAddOns = []*wire.MessageAddOn{
		{MessageAddOnType: wire.MessageAddOn_REACTION.Enum(), MessageAddOnKey: key(false, ""), LegacyMessage: &wire.LegacyMessage{PollVote: picked("Sushi")}},
		{MessageAddOnType: wire.MessageAddOn_POLL_UPDATE.Enum(), MessageAddOnKey: key(false, ""), LegacyMessage: &wire.LegacyMessage{PollVote: picked("Sushi")}, SenderTimestampMs: new(int64(1790000008000))},
	}
	addOns.Message.PollUpdates = []*wire.PollUpdate{{PollUpdateMessageKey: key(true, ""), Vote: picked("Pizza"), SenderTimestampMs: new(int64(1))}}
	vote := webMessage(bob.String(), false, "V1", "", 1790000009, &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{PollCreationMessageKey: key(true, "")}})
	text := webMessage(bob.String(), false, "T1", "", 1790000010, &wire.Message{Conversation: new("hi")})
	text.Message.PollUpdates = []*wire.PollUpdate{{PollUpdateMessageKey: key(false, ""), Vote: picked("Pizza")}}
	sync := &wire.HistorySync{SyncType: wire.HistorySync_RECENT.Enum(), Conversations: []*wire.Conversation{
		{Id: new(family.String()), Messages: []*wire.HistorySyncMsg{updates}},
		{Id: new(bob.String()), Messages: []*wire.HistorySyncMsg{addOns, vote, text}},
	}}
	data, err := proto.Marshal(sync)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := history.Parse(data, self)
	if err != nil || len(chunk.Messages) != 3 {
		t.Fatalf("Parse() = %+v, %v", chunk.Messages, err)
	}
	want := [][]history.Vote{
		{
			{By: carol, Options: []string{"Sushi"}, Time: time.UnixMilli(1790000001000)},
			{By: self, Options: []string{"Pizza"}, Time: time.UnixMilli(1790000002000)},
			{By: bob, Options: nil, Time: time.UnixMilli(1790000007000)},
		},
		{{By: bob, Options: []string{"Sushi"}, Time: time.UnixMilli(1790000008000)}},
		nil,
	}
	for i, m := range chunk.Messages {
		if len(m.Votes) != len(want[i]) {
			t.Fatalf("message %s votes = %+v", m.ID, m.Votes)
		}
		for j, v := range m.Votes {
			if v.By != want[i][j].By || !slices.Equal(v.Options, want[i][j].Options) || !v.Time.Equal(want[i][j].Time) {
				t.Fatalf("message %s vote %d = %+v, want %+v", m.ID, j, v, want[i][j])
			}
		}
	}
	if secret := chunk.Messages[0].Message.GetMessageContextInfo().GetMessageSecret(); !bytes.Equal(secret, bytes.Repeat([]byte{5}, 32)) {
		t.Fatalf("the poll's secret was not kept: %x", secret)
	}
	if info := chunk.Messages[2].Message.GetMessageContextInfo(); info != nil {
		t.Fatalf("a message without a secret gained context info: %v", info)
	}
	own := bytes.Repeat([]byte{6}, 32)
	withOwn := webMessage(bob.String(), false, "P3", "", 1790000000, &wire.Message{PollCreationMessage: poll.GetPollCreationMessageV3(), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: own}})
	withOwn.Message.MessageSecret = bytes.Repeat([]byte{7}, 32)
	data, err = proto.Marshal(&wire.HistorySync{SyncType: wire.HistorySync_RECENT.Enum(), Conversations: []*wire.Conversation{{Id: new(bob.String()), Messages: []*wire.HistorySyncMsg{withOwn}}}})
	if err != nil {
		t.Fatal(err)
	}
	if chunk, err := history.Parse(data, self); err != nil || len(chunk.Messages) != 1 || !bytes.Equal(chunk.Messages[0].Message.GetMessageContextInfo().GetMessageSecret(), own) {
		t.Fatalf("the message's own secret must win: %+v, %v", chunk.Messages, err)
	}
}

func sameChat(a, b history.Chat) bool {
	return a.JID == b.JID && a.Name == b.Name && a.Unread == b.Unread && a.LastMessage.Equal(b.LastMessage) && a.Archived == b.Archived &&
		a.Pinned == b.Pinned && a.ReadOnly == b.ReadOnly && a.MutedUntil.Equal(b.MutedUntil) && a.Token.Contact == b.Token.Contact &&
		bytes.Equal(a.Token.Theirs, b.Token.Theirs) && a.Token.Given.Equal(b.Token.Given) && a.Token.Ours.Equal(b.Token.Ours) &&
		a.Timer.Seconds == b.Timer.Seconds && a.Timer.Set.Equal(b.Timer.Set)
}
