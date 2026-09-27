package messenger

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"fmt"
	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

var (
	me     = node.JID{User: "40711111111", Server: node.ServerUser}
	myLID  = node.JID{User: "100000000000001", Server: node.ServerLID}
	bob    = node.JID{User: "40722222222", Server: node.ServerUser}
	bobLID = node.JID{User: "100000000000002", Server: node.ServerLID}
	dan    = node.JID{User: "40744444444", Server: node.ServerUser}
	family = node.JID{User: "120363000000000081", Server: node.ServerGroup}
)

func messenger(t *testing.T, now time.Time, account pairing.Account) (*Messenger, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Apply(ctx, store.Changes{LIDs: map[node.JID]node.JID{bobLID: bob, myLID: me}}); err != nil {
		t.Fatal(err)
	}
	m := New(linkflow.Config{Now: func() time.Time { return now }}, nil, nil, st)
	m.state = &client.State{Linked: linkflow.Linked{Account: account}}
	return m, ctx
}

func lunch(selectable uint32) *wire.Message_PollCreationMessage {
	return &wire.Message_PollCreationMessage{Name: new("Lunch?"), SelectableOptionsCount: new(selectable), Options: []*wire.Message_PollCreationMessage_Option{
		{OptionName: new("Pizza")}, {OptionName: new("Sushi")}, {OptionName: new(" Soup ")},
	}}
}

func TestChoosingPollOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		limit   uint32
		wanted  []string
		want    []string
		wantErr error
	}{
		{name: "exact", limit: 1, wanted: []string{"Sushi"}, want: []string{"Sushi"}},
		{name: "any case and spacing", limit: 1, wanted: []string{"  pIZZA "}, want: []string{"Pizza"}},
		{name: "the poll's own spacing is kept", limit: 1, wanted: []string{"soup"}, want: []string{" Soup "}},
		{name: "the same option twice counts once", limit: 1, wanted: []string{"Pizza", "pizza"}, want: []string{"Pizza"}},
		{name: "as many as allowed", limit: 2, wanted: []string{"Soup", "Pizza"}, want: []string{" Soup ", "Pizza"}},
		{name: "no limit", limit: 0, wanted: []string{"Pizza", "Sushi", "Soup"}, want: []string{"Pizza", "Sushi", " Soup "}},
		{name: "over the limit", limit: 2, wanted: []string{"Pizza", "Sushi", "Soup"}, wantErr: ErrTooMany},
		{name: "an unknown option", limit: 0, wanted: []string{"Pizza", "Tacos"}, wantErr: ErrNoOption},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := choose(lunch(tt.limit), tt.wanted)
			if !errors.Is(err, tt.wantErr) || !slices.Equal(got, tt.want) {
				t.Fatalf("choose() = %q, %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestVotingPicksOneAddressingForBothSides(t *testing.T) {
	t.Parallel()
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	strangerLID := node.JID{User: "100000000000009", Server: node.ServerLID}
	tests := []struct {
		name        string
		account     pairing.Account
		creator     node.JID
		wantCreator node.JID
		wantVoter   node.JID
	}{
		{name: "both have lids", account: pairing.Account{JID: me, LID: myLID}, creator: bob, wantCreator: bobLID, wantVoter: myLID},
		{name: "a creator's device is dropped", account: pairing.Account{JID: me, LID: myLID}, creator: node.JID{User: bob.User, Server: bob.Server, Device: 4}, wantCreator: bobLID, wantVoter: myLID},
		{name: "creator without a lid", account: pairing.Account{JID: me, LID: myLID}, creator: carol, wantCreator: carol, wantVoter: me},
		{name: "we have no lid", account: pairing.Account{JID: me}, creator: bobLID, wantCreator: bob, wantVoter: me},
		{name: "our own poll", account: pairing.Account{JID: me, LID: myLID}, creator: me, wantCreator: myLID, wantVoter: myLID},
		{name: "nothing matches", account: pairing.Account{JID: me}, creator: strangerLID, wantCreator: strangerLID, wantVoter: me},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, ctx := messenger(t, time.Unix(1790000000, 0), tt.account)
			creator, voter, err := m.ballotAddresses(ctx, tt.creator)
			if err != nil || creator != tt.wantCreator || voter != tt.wantVoter {
				t.Fatalf("ballotAddresses() = %v, %v, %v; want %v, %v", creator, voter, err, tt.wantCreator, tt.wantVoter)
			}
		})
	}
}

func TestVotesOpenUnderAnyAddressOfTheirSenders(t *testing.T) {
	t.Parallel()
	at := time.Unix(1790000000, 0)
	m, ctx := messenger(t, at, pairing.Account{JID: me, LID: myLID})
	secret := bytes.Repeat([]byte{2}, 32)
	poll := &wire.Message{PollCreationMessageV3: lunch(1), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: secret}}
	m.received(client.Received{ID: "P1", Chat: family, Author: bob, Time: at, Edit: message.EditNone, Message: poll})
	m.received(client.Received{ID: "T1", Chat: family, Author: bob, Time: at, Edit: message.EditNone, Message: &wire.Message{Conversation: new("not a poll"), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: secret}}})
	cast := func(target string, from node.JID, sealedAs message.Ballot, options ...string) {
		t.Helper()
		enc, err := message.SealVote(rand.Reader, sealedAs, options)
		if err != nil {
			t.Fatal(err)
		}
		m.received(client.Received{ID: "V-" + from.String(), Chat: family, Author: from, Time: at.Add(time.Minute), Edit: message.EditNone, Message: &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{
			PollCreationMessageKey: &wire.MessageKey{Id: new(target)}, Vote: enc, SenderTimestampMs: new(at.Add(time.Minute).UnixMilli()),
		}}})
	}
	ballot := func(creator, voter node.JID) message.Ballot {
		return message.Ballot{Secret: secret, PollID: "P1", Creator: creator, Voter: voter}
	}
	cast("P1", node.JID{User: bob.User, Server: bob.Server, Device: 5}, ballot(bobLID, bobLID), "Sushi")
	cast("P1", myLID, ballot(bob, me), "Pizza")
	cast("P1", dan, ballot(bob, dan), " Soup ")
	cast("P1", node.JID{User: "40755555555", Server: node.ServerUser}, message.Ballot{Secret: bytes.Repeat([]byte{7}, 32), PollID: "P1", Creator: bob, Voter: node.JID{User: "40755555555", Server: node.ServerUser}}, "Pizza")
	cast("P1", node.JID{User: "40766666666", Server: node.ServerUser}, ballot(bob, node.JID{User: "40766666666", Server: node.ServerUser}), "Pizza", "Sushi")
	cast("T1", dan, message.Ballot{Secret: secret, PollID: "T1", Creator: bob, Voter: dan}, "Pizza")
	cast("NOPE", dan, message.Ballot{Secret: secret, PollID: "NOPE", Creator: bob, Voter: dan}, "Pizza")
	got, ok, err := m.store.MessageIn(ctx, family, "P1")
	if err != nil || !ok {
		t.Fatalf("poll = %v, %v", ok, err)
	}
	votes := map[string][]string{}
	for _, v := range got.Votes {
		votes[v.By.String()] = v.Options
	}
	want := map[string][]string{bob.String(): {"Sushi"}, me.String(): {"Pizza"}, dan.String(): {" Soup "}}
	if len(votes) != len(want) {
		t.Fatalf("votes = %v, want %v", votes, want)
	}
	for by, options := range want {
		if !slices.Equal(votes[by], options) {
			t.Fatalf("votes = %v, want %v", votes, want)
		}
	}
	all, err := m.store.Messages(ctx, store.Query{Chat: family, Limit: 10})
	if err != nil || len(all) != 2 {
		t.Fatalf("votes must not be stored as messages: %d, %v", len(all), err)
	}
	if text, _, _ := m.store.MessageIn(ctx, family, "T1"); len(text.Votes) != 0 {
		t.Fatalf("a vote on a message that is not a poll was kept: %+v", text.Votes)
	}
}

func TestOnlyOwnRecentMessagesChange(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790000000, 0)
	m, ctx := messenger(t, now, pairing.Account{JID: me, LID: myLID})
	say := func(id string, chat, author node.JID, sent time.Time, msg *wire.Message) {
		m.received(client.Received{ID: id, Chat: chat, Author: author, Time: sent, Edit: message.EditNone, Message: msg})
	}
	text := func(s string) *wire.Message { return &wire.Message{Conversation: new(s)} }
	say("fresh", bob, me, now.Add(-editWindow), text("hi"))
	say("old", bob, me, now.Add(-editWindow-time.Second), text("hi"))
	say("ancient", bob, me, now.Add(-deleteWindow-time.Second), text("hi"))
	say("theirs", family, bob, now, text("hi"))
	say("bobs", bob, bob, now, text("hi"))
	say("story", node.StatusBroadcast(), bob, now, text("at the beach"))
	say("photo", bob, me, now, &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sea")}})
	say("voice", bob, me, now, &wire.Message{AudioMessage: &wire.Message_AudioMessage{Ptt: new(true)}})
	say("gone", bob, me, now, text("oops"))
	say("revoke", bob, me, now, &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: &wire.MessageKey{Id: new("gone")}, Type: wire.Message_ProtocolMessage_REVOKE.Enum()}})

	for _, tt := range []struct {
		id      string
		window  time.Duration
		wantErr error
	}{
		{id: "fresh", window: editWindow},
		{id: "old", window: editWindow, wantErr: ErrTooLate},
		{id: "old", window: deleteWindow},
		{id: "ancient", window: deleteWindow, wantErr: ErrTooLate},
		{id: "theirs", window: deleteWindow, wantErr: ErrNotMine},
		{id: "gone", window: deleteWindow, wantErr: ErrDeleted},
		{id: "missing", window: deleteWindow, wantErr: ErrUnknownMessage},
	} {
		if _, _, err := m.own(ctx, tt.id, tt.window); !errors.Is(err, tt.wantErr) {
			t.Errorf("own(%s, %s) error = %v, want %v", tt.id, tt.window, err, tt.wantErr)
		}
	}
	for _, tt := range []struct {
		id   string
		want string
	}{
		{id: "fresh", want: "hi"},
		{id: "bobs"},
		{id: "gone"},
		{id: "missing"},
	} {
		sent, ok := m.sentMessage(ctx, bob, tt.id)
		if ok != (tt.want != "") || sent.GetConversation() != tt.want {
			t.Errorf("sentMessage(%s) = %v, %v; only our own undeleted messages are sent again", tt.id, sent, ok)
		}
	}
	if _, key, err := m.target(ctx, "theirs"); err != nil || key.GetParticipant() != bob.String() || key.GetFromMe() || key.GetRemoteJid() != family.String() || key.GetId() != "theirs" {
		t.Fatalf("someone else's group message: %v, %v", key, err)
	}
	if _, key, err := m.target(ctx, "fresh"); err != nil || key.Participant != nil || !key.GetFromMe() || key.GetRemoteJid() != bob.String() {
		t.Fatalf("our own message: %v, %v", key, err)
	}
	if _, key, err := m.target(ctx, "bobs"); err != nil || key.Participant != nil || key.GetFromMe() || key.GetRemoteJid() != bob.String() {
		t.Fatalf("someone else's message one to one: %v, %v", key, err)
	}
	if _, key, err := m.target(ctx, "story"); err != nil || key.GetParticipant() != bob.String() || key.GetRemoteJid() != "status@broadcast" {
		t.Fatalf("someone else's status: %v, %v", key, err)
	}
	short, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	for _, tt := range []struct {
		name    string
		id      string
		text    string
		wantErr error
	}{
		{name: "empty text", id: "fresh", text: " \n", wantErr: ErrEmptyEdit},
		{name: "a voice note", id: "voice", text: "new caption", wantErr: ErrNotText},
		{name: "a photo's caption", id: "photo", text: "new caption", wantErr: context.DeadlineExceeded},
		{name: "a text", id: "fresh", text: "hello", wantErr: context.DeadlineExceeded},
	} {
		if _, err := m.Edit(short, tt.id, tt.text); !errors.Is(err, tt.wantErr) {
			t.Errorf("Edit(%s) error = %v, want %v", tt.name, err, tt.wantErr)
		}
	}
	for _, emoji := range []string{"not an emoji", "👍👍", "a"} {
		if _, err := m.React(short, "bobs", emoji); !errors.Is(err, ErrNotEmoji) {
			t.Errorf("React(%q) error = %v, want %v", emoji, err, ErrNotEmoji)
		}
	}
	if _, err := m.React(short, "bobs", "👍🏽"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("React(👍🏽) error = %v, want it sent", err)
	}
	if _, _, err := m.Vote(ctx, "fresh", []string{"Pizza"}); !errors.Is(err, ErrNotPoll) {
		t.Errorf("voting on a text: %v", err)
	}
}

func TestChatFlagsFromTheAppState(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(1790000000000)
	set := func(value *wire.SyncActionValue, index ...string) appstate.Mutation {
		return appstate.Mutation{Operation: wire.SyncdMutation_SET, Index: index, Value: value}
	}
	pin := func(pinned *bool, ms int64) *wire.SyncActionValue {
		return &wire.SyncActionValue{Timestamp: new(ms), PinAction: &wire.SyncActionValue_PinAction{Pinned: pinned}}
	}
	mute := func(muted *bool, end *int64) *wire.SyncActionValue {
		return &wire.SyncActionValue{MuteAction: &wire.SyncActionValue_MuteAction{Muted: muted, MuteEndTimestamp: end}}
	}
	archive := func(archived *bool) *wire.SyncActionValue {
		return &wire.SyncActionValue{ArchiveChatAction: &wire.SyncActionValue_ArchiveChatAction{Archived: archived}}
	}
	chat := func(n int) string {
		return node.JID{User: "4072000000" + strconv.Itoa(n), Server: node.ServerUser}.String()
	}
	got := chatFlagsFrom([]appstate.Mutation{
		set(pin(new(true), 1790000001000), "pin_v1", chat(1)),
		set(pin(new(true), 0), "pin_v1", chat(2)),
		set(pin(new(false), 1790000002000), "pin_v1", chat(3)),
		set(pin(nil, 1790000002000), "pin_v1", chat(4)),
		set(&wire.SyncActionValue{}, "pin_v1", chat(5)),
		{Operation: wire.SyncdMutation_REMOVE, Index: []string{"pin_v1", chat(6)}, Value: pin(new(true), 1)},
		set(pin(new(true), 1), "pin_v1", "not a jid"),
		set(pin(new(true), 1), "pin_v1"),
		set(archive(new(true)), "archive", chat(1)),
		set(archive(new(false)), "archive", chat(2)),
		set(archive(nil), "archive", chat(3)),
		set(mute(new(true), new(int64(-1))), "mute", chat(1)),
		set(mute(new(true), new(int64(1790003600500))), "mute", chat(2)),
		set(mute(new(true), new(int64(1789999999000))), "mute", chat(3)),
		set(mute(new(false), nil), "mute", chat(4)),
		set(mute(new(true), nil), "mute", chat(5)),
		set(mute(nil, new(int64(-1))), "mute", chat(6)),
		set(mute(new(true), new(int64(-500))), "mute", chat(7)),
		set(mute(new(true), new(int64(-2000))), "mute", chat(8)),
		set(mute(new(true), new(int64(-1000))), "mute", "40730000001@s.whatsapp.net"),
		set(mute(new(true), new(int64(-1001))), "mute", "40730000002@s.whatsapp.net"),
		set(mute(new(true), new(int64(1790000000000))), "mute", "40730000003@s.whatsapp.net"),
		set(mute(new(true), new(int64(1790000000999))), "mute", "40730000004@s.whatsapp.net"),
		set(mute(new(false), new(int64(1790003600000))), "mute", chat(9)),
		set(mute(new(true), new(int64(0))), "mute", "40720000000@s.whatsapp.net"),
		set(archive(new(true)), "star", chat(9)),
	}, now)
	jid := func(s string) node.JID {
		j, err := node.ParseJID(s)
		if err != nil {
			t.Fatal(err)
		}
		return j
	}
	wantPins := map[node.JID]time.Time{jid(chat(1)): time.UnixMilli(1790000001000), jid(chat(2)): time.UnixMilli(1), jid(chat(3)): {}}
	wantArchives := map[node.JID]bool{jid(chat(1)): true, jid(chat(2)): false}
	wantMutes := map[node.JID]store.Mute{
		jid(chat(1)): store.MuteForever, jid(chat(2)): 1790003600, jid(chat(3)): 0, jid(chat(4)): 0,
		jid(chat(7)): store.MuteForever, jid(chat(8)): 0, jid(chat(9)): 1790003600, jid("40720000000@s.whatsapp.net"): 0,
		jid("40730000001@s.whatsapp.net"): store.MuteForever, jid("40730000002@s.whatsapp.net"): 0,
		jid("40730000003@s.whatsapp.net"): 1790000000, jid("40730000004@s.whatsapp.net"): 1790000000,
	}
	if !maps.EqualFunc(got.Pins, wantPins, time.Time.Equal) || !maps.Equal(got.Archives, wantArchives) || !maps.Equal(got.Mutes, wantMutes) {
		t.Fatalf("pins %v\narchives %v\nmutes %v", got.Pins, got.Archives, got.Mutes)
	}
}

func TestPacingProtectsTheAccount(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790100000, 0)
	m, ctx := messenger(t, now, pairing.Account{JID: me, LID: myLID})
	seed := make([]store.Message, 0, MaxNewChats+1)
	for i := range MaxNewChats {
		to := node.JID{User: "4079900000" + strconv.Itoa(i), Server: node.ServerUser}
		seed = append(seed, store.Message{ID: "n" + strconv.Itoa(i), Chat: to, Author: me, FromMe: true, Time: now.Add(-time.Hour), Message: &wire.Message{Conversation: new("hi")}})
	}
	seed = append(seed, store.Message{ID: "b1", Chat: bob, Author: bob, Time: now.Add(-2 * time.Hour), Message: &wire.Message{Conversation: new("hey")}})
	if err := m.store.Apply(ctx, store.Changes{Messages: seed}); err != nil {
		t.Fatal(err)
	}
	stranger := node.JID{User: "40788888888", Server: node.ServerUser}
	for _, tt := range []struct {
		name    string
		to      node.JID
		wantErr error
	}{
		{name: "one more new chat", to: stranger, wantErr: ErrNewChats},
		{name: "a chat we have", to: bob},
		{name: "a group", to: family},
		{name: "ourselves", to: me},
		{name: "ourselves by lid", to: myLID},
	} {
		if err := m.pace(ctx, tt.to); !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: pace() = %v, want %v", tt.name, err, tt.wantErr)
		}
	}
	burst := make([]store.Message, 0, MaxPerMinute)
	for i := range MaxPerMinute {
		burst = append(burst, store.Message{ID: "f" + strconv.Itoa(i), Chat: bob, Author: me, FromMe: true, Time: now.Add(-time.Duration(i) * time.Second), Message: &wire.Message{Conversation: new("x")}})
	}
	if err := m.store.Apply(ctx, store.Changes{Messages: burst[:MaxPerMinute-1]}); err != nil {
		t.Fatal(err)
	}
	if err := m.pace(ctx, bob); err != nil {
		t.Fatalf("one below the limit: %v", err)
	}
	if err := m.store.Apply(ctx, store.Changes{Messages: burst[MaxPerMinute-1:]}); err != nil {
		t.Fatal(err)
	}
	if err := m.pace(ctx, family); !errors.Is(err, ErrTooFast) {
		t.Fatalf("at the limit: %v", err)
	}
}

func TestHistorySyncProgress(t *testing.T) {
	t.Parallel()
	m, _ := messenger(t, time.Unix(1790000000, 0), pairing.Account{JID: me})
	if got := m.Connection().History; got.Started {
		t.Fatalf("before any history: %+v", got)
	}
	for _, tt := range []struct {
		kind    wire.HistorySync_HistorySyncType
		percent uint32
		want    HistorySync
	}{
		{wire.HistorySync_PUSH_NAME, 90, HistorySync{}},
		{wire.HistorySync_INITIAL_BOOTSTRAP, 10, HistorySync{Started: true, Percent: 10}},
		{wire.HistorySync_RECENT, 40, HistorySync{Started: true, Percent: 40}},
		{wire.HistorySync_RECENT, 30, HistorySync{Started: true, Percent: 40}},
		{wire.HistorySync_ON_DEMAND, 0, HistorySync{Started: true, Percent: 40}},
		{wire.HistorySync_RECENT, 180, HistorySync{Started: true, Percent: 100}},
	} {
		m.progress(tt.kind, tt.percent)
		if got := m.Connection().History; got != tt.want {
			t.Fatalf("after %s %d%%: %+v, want %+v", tt.kind, tt.percent, got, tt.want)
		}
	}
	fresh, _ := messenger(t, time.Unix(1790000000, 0), pairing.Account{JID: me})
	fresh.progress(wire.HistorySync_FULL, 5)
	if got := fresh.Connection().History; got != (HistorySync{Started: true, Percent: 100}) {
		t.Fatalf("a full sync: %+v", got)
	}
}

func TestWhatCannotBeForwarded(t *testing.T) {
	t.Parallel()
	photo := &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sea")}}
	future := func(m *wire.Message) *wire.Message_FutureProofMessage {
		return &wire.Message_FutureProofMessage{Message: m}
	}
	for _, tt := range []struct {
		name string
		msg  *wire.Message
		want bool
	}{
		{"a photo", photo, false},
		{"a text", &wire.Message{Conversation: new("hi")}, false},
		{"view once", &wire.Message{ViewOnceMessage: future(photo)}, true},
		{"view once v2", &wire.Message{ViewOnceMessageV2: future(photo)}, true},
		{"view once v2 extension", &wire.Message{ViewOnceMessageV2Extension: future(photo)}, true},
		{"view once inside a disappearing message", &wire.Message{EphemeralMessage: future(&wire.Message{ViewOnceMessageV2: future(photo)})}, true},
		{"a view once photo", &wire.Message{ImageMessage: &wire.Message_ImageMessage{ViewOnce: new(true)}}, true},
		{"a view once video", &wire.Message{VideoMessage: &wire.Message_VideoMessage{ViewOnce: new(true)}}, true},
		{"a view once voice note", &wire.Message{AudioMessage: &wire.Message_AudioMessage{ViewOnce: new(true)}}, true},
	} {
		if got := viewOnce(tt.msg); got != tt.want {
			t.Errorf("%s: viewOnce() = %v", tt.name, got)
		}
	}
	m, ctx := messenger(t, time.Unix(1790000000, 0), pairing.Account{JID: me})
	m.received(client.Received{ID: "P1", Chat: bob, Author: bob, Time: time.Unix(1790000000, 0), Edit: message.EditNone, Message: &wire.Message{PollCreationMessageV3: lunch(1)}})
	m.received(client.Received{ID: "O1", Chat: bob, Author: bob, Time: time.Unix(1790000000, 0), Edit: message.EditNone, Message: &wire.Message{ViewOnceMessageV2: future(photo)}})
	for _, id := range []string{"P1", "O1"} {
		if _, _, err := m.Forward(ctx, family, id); !errors.Is(err, ErrNoForward) {
			t.Errorf("Forward(%s) = %v, want ErrNoForward", id, err)
		}
	}
	if _, _, err := m.Forward(ctx, family, "NOPE"); !errors.Is(err, ErrUnknownMessage) {
		t.Errorf("Forward(unknown) = %v", err)
	}
}

func TestTheChatOfAMessageAndMentionAddresses(t *testing.T) {
	t.Parallel()
	m, ctx := messenger(t, time.Unix(1790000000, 0), pairing.Account{JID: me})
	m.received(client.Received{ID: "G1", Chat: family, Author: bob, Time: time.Unix(1790000000, 0), Edit: message.EditNone, Message: &wire.Message{Conversation: new("hi")}})
	if chat, err := m.ChatOf(ctx, "G1"); err != nil || chat != family {
		t.Fatalf("ChatOf() = %v, %v", chat, err)
	}
	if _, err := m.ChatOf(ctx, "NOPE"); !errors.Is(err, ErrUnknownMessage) {
		t.Fatalf("ChatOf(unknown) = %v", err)
	}
	withDevice := node.JID{User: bob.User, Server: bob.Server, Device: 3}
	if got := addresses([]node.JID{withDevice, myLID}); !slices.Equal(got, []string{bob.String(), myLID.String()}) {
		t.Fatalf("addresses() = %v", got)
	}
}

func TestTheHoldAfterA463SparesChatsWeHave(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790100000, 0)
	m, ctx := messenger(t, now, pairing.Account{JID: me, LID: myLID})
	if err := m.store.Apply(ctx, store.Changes{Messages: []store.Message{{ID: "b1", Chat: bob, Author: bob, Time: now.Add(-time.Hour), Message: &wire.Message{Conversation: new("hey")}}}}); err != nil {
		t.Fatal(err)
	}
	m.limited = now.Add(restrictedFor)
	stranger := node.JID{User: "40788888888", Server: node.ServerUser}
	for _, tt := range []struct {
		name    string
		to      node.JID
		wantErr error
	}{
		{name: "a new contact", to: stranger, wantErr: ErrRestricted},
		{name: "a chat we have", to: bob},
		{name: "a group", to: family},
		{name: "ourselves", to: me},
	} {
		if err := m.pace(ctx, tt.to); !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: pace() = %v, want %v", tt.name, err, tt.wantErr)
		}
	}
	m.link.Now = func() time.Time { return now.Add(restrictedFor) }
	if err := m.pace(ctx, stranger); err != nil {
		t.Fatalf("a day later: %v", err)
	}
}

func TestEncryptedEditsChangeTheOriginalInPlace(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790000000, 0)
	m, ctx := messenger(t, now, pairing.Account{JID: me, LID: myLID})
	secret := bytes.Repeat([]byte{7}, message.SecretSize)
	original := &wire.Message{Conversation: new("see you at 7"), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: secret}}
	m.received(client.Received{ID: "3EB0ORIG", Chat: bob, Author: bob, Time: now, Message: original})
	sealed, err := message.SealEdit(rand.Reader, message.Addon{Secret: secret, ID: "3EB0ORIG", Original: bobLID, Sender: bobLID}, &wire.Message{Conversation: new("see you at 8")})
	if err != nil {
		t.Fatal(err)
	}
	m.received(client.Received{ID: "3EB0EDIT", Chat: bobLID, Author: bobLID, Time: now.Add(time.Minute), Message: sealed})
	forged, err := message.SealEdit(rand.Reader, message.Addon{Secret: secret, ID: "3EB0ORIG", Original: bobLID, Sender: bobLID}, &wire.Message{Conversation: new("forged")})
	if err != nil {
		t.Fatal(err)
	}
	m.received(client.Received{ID: "3EB0FORGED", Chat: bob, Author: node.JID{User: "40799999999", Server: node.ServerUser}, Time: now.Add(2 * time.Minute), Message: forged})
	all, err := m.store.Messages(ctx, store.Query{Chat: bob, Limit: 10})
	if err != nil || len(all) != 1 {
		t.Fatalf("the edits were stored as messages: %d rows, %v", len(all), err)
	}
	if got := store.Text(all[0].Message); got != "see you at 8" || all[0].Edited.IsZero() {
		t.Fatalf("after the encrypted edit: %q, edited %v", got, all[0].Edited)
	}
}

func TestAskingThePhoneForOlderMessages(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790000000, 0)
	m, ctx := messenger(t, now, pairing.Account{JID: me, LID: myLID})
	for i, text := range []string{"third", "fourth"} {
		m.received(client.Received{ID: fmt.Sprint("3EB0N", i), Chat: bob, Author: bob, Time: now.Add(time.Duration(i) * time.Minute), Message: &wire.Message{Conversation: new(text)}})
	}
	var asked *wire.Message_PeerDataOperationRequestMessage_HistorySyncOnDemandRequest
	n, err := m.older(ctx, bob, func(_ context.Context, request *wire.Message) error {
		asked = request.GetProtocolMessage().GetPeerDataOperationRequestMessage().GetHistorySyncOnDemandRequest()
		go m.history(history.Chunk{Type: wire.HistorySync_ON_DEMAND, Messages: []history.Message{
			{ID: "3EB0O1", Chat: bob, Author: bob, Time: now.Add(-2 * time.Hour), Message: &wire.Message{Conversation: new("first")}},
			{ID: "3EB0O2", Chat: bob, Author: me, FromMe: true, Time: now.Add(-time.Hour), Message: &wire.Message{Conversation: new("second")}},
		}})
		return nil
	})
	if err != nil || n != 2 {
		t.Fatalf("older() = %d, %v; want the 2 messages the phone sent", n, err)
	}
	if asked.GetChatJid() != bob.String() || asked.GetOldestMsgId() != "3EB0N0" || asked.GetOldestMsgFromMe() || asked.GetOldestMsgTimestampMs() != now.UnixMilli() || asked.GetOnDemandMsgCount() != olderCount {
		t.Fatalf("asked the phone for %v", asked)
	}
	all, err := m.store.Messages(ctx, store.Query{Chat: bob, Limit: 10})
	if err != nil || len(all) != 4 || store.Text(all[0].Message) != "first" {
		t.Fatalf("the chat now holds %d messages, %v", len(all), err)
	}
	start := time.Now()
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if n, err := m.older(short, bob, func(context.Context, *wire.Message) error { return nil }); n != 0 || err != nil || time.Since(start) > time.Second {
		t.Fatalf("a phone that never answers: %d, %v after %s", n, err, time.Since(start))
	}
}
