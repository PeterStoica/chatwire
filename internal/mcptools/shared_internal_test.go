package mcptools

import (
	"bytes"
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestSharedContentReadsAsWhatItIs(t *testing.T) {
	t.Parallel()
	dan := &wire.Message_ContactMessage{DisplayName: new("Dan"), Vcard: new("BEGIN:VCARD\nVERSION:3.0\nFN:Dan\nTEL;type=CELL;waid=40712345678:+40 712 345 678\nEND:VCARD")}
	ana := &wire.Message_ContactMessage{DisplayName: new("Ana"), Vcard: new("BEGIN:VCARD\r\nitem1.TEL;waid=40733333333:+40 733 333 333\r\nitem1.X-ABLabel:Mobile\r\nEND:VCARD")}
	tests := []struct {
		name     string
		msg      *wire.Message
		wantKind string
		wantBody string
	}{
		{
			name:     "named place",
			msg:      &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(44.4268), DegreesLongitude: new(26.1025), Name: new("Home"), Address: new(" Str. X 1 "), Url: new("https://maps.example/h")}},
			wantKind: "location",
			wantBody: "[location 44.4268, 26.1025] Home, Str. X 1, https://maps.example/h",
		},
		{
			name:     "pin without a name",
			msg:      &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(-33.8688), DegreesLongitude: new(151.2093)}},
			wantKind: "location",
			wantBody: "[location -33.8688, 151.2093]",
		},
		{
			name:     "location with a comment",
			msg:      &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(1.5), DegreesLongitude: new(2.0), Comment: new("gate B")}},
			wantKind: "location",
			wantBody: "[location 1.5, 2] gate B",
		},
		{
			name:     "location flagged live",
			msg:      &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(1.0), DegreesLongitude: new(2.0), IsLive: new(true)}},
			wantKind: "live_location",
			wantBody: "[live location 1, 2]",
		},
		{
			name:     "live location",
			msg:      &wire.Message{LiveLocationMessage: &wire.Message_LiveLocationMessage{DegreesLatitude: new(45.75), DegreesLongitude: new(21.23), Caption: new("on my way")}},
			wantKind: "live_location",
			wantBody: "[live location 45.75, 21.23] on my way",
		},
		{
			name:     "contact card",
			msg:      &wire.Message{ContactMessage: dan},
			wantKind: "contact",
			wantBody: "[contact] Dan: +40 712 345 678",
		},
		{
			name:     "contact without a name or number",
			msg:      &wire.Message{ContactMessage: &wire.Message_ContactMessage{DisplayName: new("  ")}},
			wantKind: "contact",
			wantBody: "[contact] unnamed",
		},
		{
			name:     "several contacts",
			msg:      &wire.Message{ContactsArrayMessage: &wire.Message_ContactsArrayMessage{DisplayName: new("2 contacts"), Contacts: []*wire.Message_ContactMessage{dan, ana}}},
			wantKind: "contact",
			wantBody: "[contacts] Dan: +40 712 345 678; Ana: +40 733 333 333",
		},
		{
			name:     "single choice poll",
			msg:      &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}}}},
			wantKind: "poll",
			wantBody: "[poll] Lunch? Options: Pizza / Sushi",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := describeMessage(directory{}, store.Message{ID: "3EB0", Chat: node.JID{User: "40722222222", Server: node.ServerUser}, Message: tt.msg})
			if got.Kind != tt.wantKind || got.Text != tt.wantBody {
				t.Fatalf("describeMessage() = %q %q, want %q %q", got.Kind, got.Text, tt.wantKind, tt.wantBody)
			}
		})
	}
}

func TestPhoneNumbersFromVCards(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		vcard string
		want  []string
	}{
		{name: "empty", vcard: "", want: nil},
		{name: "whatsapp card", vcard: "BEGIN:VCARD\nTEL;type=CELL;type=VOICE;waid=40712345678:+40 712 345 678\nEND:VCARD", want: []string{"+40 712 345 678"}},
		{name: "several numbers and line endings", vcard: "TEL:+1 555 0100\r\ntel;TYPE=home:+1 555 0101\r\n", want: []string{"+1 555 0100", "+1 555 0101"}},
		{name: "grouped property", vcard: "item2.TEL:+44 20 7946 0000", want: []string{"+44 20 7946 0000"}},
		{name: "uri value", vcard: "TEL;VALUE=uri:tel:+33 1 23 45 67 89", want: []string{"+33 1 23 45 67 89"}},
		{name: "other properties", vcard: "FN:Tel Aviv Office\nEMAIL:tel@example.com\nTELX:+1\nX-TEL:+2\nNOTE:TEL:+3", want: nil},
		{name: "blank or missing value", vcard: "TEL:\nTEL;waid=1:  \nTEL", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := phones(tt.vcard); !slices.Equal(got, tt.want) {
				t.Fatalf("phones() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPollsShowWhoVotedForWhat(t *testing.T) {
	t.Parallel()
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	myLID := node.JID{User: "100000000000001", Server: node.ServerLID}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	carolLID := node.JID{User: "100000000000003", Server: node.ServerLID}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	dan := node.JID{User: "40744444444", Server: node.ServerUser, Device: 2}
	dir := directory{
		self:  me,
		names: map[node.JID]store.Name{bob: {Contact: "Bob"}},
		lids:  map[node.JID]node.JID{myLID: me, carolLID: carol},
	}
	poll := &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}, {OptionName: new("Soup")}}}}
	tests := []struct {
		name  string
		votes []store.Vote
		want  string
	}{
		{name: "nobody voted", want: "[poll] Lunch? Options: Pizza / Sushi / Soup"},
		{
			name: "several voters",
			votes: []store.Vote{
				{By: bob, Options: []string{"Pizza"}},
				{By: myLID, Options: []string{"Pizza", "Sushi"}},
				{By: carolLID, Options: []string{"Sushi"}},
				{By: dan, Options: []string{"Pizza"}},
			},
			want: "[poll] Lunch? Options: Pizza (3: Bob, me, +40744444444) / Sushi (2: me, +40733333333) / Soup (0)",
		},
		{name: "a vote for an option the poll lost", votes: []store.Vote{{By: bob, Options: []string{"Tacos"}}}, want: "[poll] Lunch? Options: Pizza (0) / Sushi (0) / Soup (0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := describeMessage(dir, store.Message{ID: "3EB0", Chat: bob, Author: bob, Message: poll, Votes: tt.votes})
			if got.Kind != "poll" || got.Text != tt.want {
				t.Fatalf("describeMessage() = %q %q, want %q", got.Kind, got.Text, tt.want)
			}
		})
	}
}

func TestMentionsAndForwardsRead(t *testing.T) {
	t.Parallel()
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	carolLID := node.JID{User: "100000000000003", Server: node.ServerLID}
	family := node.JID{User: "120363000000000021", Server: node.ServerGroup}
	dir := directory{self: me, names: map[node.JID]store.Name{bob: {Contact: "Bob"}}, lids: map[node.JID]node.JID{carolLID: carol}}
	mentions := []string{bob.String(), carolLID.String(), me.String(), "not a jid", "s.whatsapp.net"}
	text := func(s string, context *wire.ContextInfo) *wire.Message {
		return &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new(s), ContextInfo: context}}
	}
	photo := &wire.Message{ImageMessage: &wire.Message_ImageMessage{
		DirectPath: new("/v/p.enc"), MediaKey: bytes.Repeat([]byte{1}, 32), Caption: new("look @40722222222"),
		ContextInfo: &wire.ContextInfo{MentionedJid: mentions},
	}}
	tests := []struct {
		name      string
		msg       *wire.Message
		revoked   bool
		want      string
		kind      string
		media     bool
		forwarded string
	}{
		{name: "names, lids and me", msg: text("@40722222222 and @100000000000003 meet @40711111111", &wire.ContextInfo{MentionedJid: mentions}), want: "@Bob and @+40733333333 meet @me"},
		{name: "numbers not mentioned stay", msg: text("call @40799999999 or @407222222229", &wire.ContextInfo{MentionedJid: mentions}), want: "call @40799999999 or @407222222229"},
		{name: "at signs that are not mentions", msg: text("mail bob@example.com @ noon @", &wire.ContextInfo{MentionedJid: mentions}), want: "mail bob@example.com @ noon @"},
		{name: "no mentions", msg: text("@40722222222", nil), want: "@40722222222"},
		{name: "caption", msg: photo, want: "look @Bob", kind: "image", media: true},
		{name: "forwarded", msg: text("hi", &wire.ContextInfo{IsForwarded: new(true), ForwardingScore: new(uint32(4))}), want: "hi", forwarded: "once"},
		{name: "forwarded many times", msg: text("hi", &wire.ContextInfo{IsForwarded: new(true), ForwardingScore: new(uint32(5))}), want: "hi", forwarded: "many times"},
		{name: "deleted", msg: text("@40722222222", &wire.ContextInfo{IsForwarded: new(true), MentionedJid: mentions}), revoked: true, want: "", kind: "deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := describeMessage(dir, store.Message{ID: "3EB0", Chat: family, Author: bob, Message: tt.msg, Revoked: tt.revoked})
			if got.Text != tt.want || got.Kind != cmp.Or(tt.kind, "text") || got.Media != tt.media || got.Forwarded != tt.forwarded {
				t.Fatalf("describeMessage() = %+v, want text %q", got, tt.want)
			}
		})
	}
}

type recording struct {
	Sender
	asked []store.Query
}

func (r *recording) Messages(_ context.Context, q store.Query) ([]store.Message, error) {
	r.asked = append(r.asked, q)
	out := make([]store.Message, q.Limit)
	for i := range out {
		out[i] = store.Message{ID: q.Chat.User + "-" + strconv.Itoa(i), Chat: q.Chat}
	}
	return out, nil
}

func TestUnreadStopsAtTheCap(t *testing.T) {
	t.Parallel()
	chat := func(user string, unread int) store.Chat {
		return store.Chat{JID: node.JID{User: user, Server: node.ServerUser}, Unread: unread}
	}
	dir := directory{chats: []store.Chat{chat("401", 150), chat("402", 0), chat("403", 100), chat("404", 5)}}
	tests := []struct {
		name  string
		only  node.JID
		want  []string
		total int
	}{
		{name: "every chat", want: []string{"401:150", "403:50"}, total: maxRead},
		{name: "one chat", only: dir.chats[3].JID, want: []string{"404:5"}, total: 5},
		{name: "a chat with nothing unread", only: dir.chats[1].JID, want: nil, total: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &recording{}
			got, err := unread(t.Context(), s, dir, tt.only)
			asked := make([]string, 0, len(s.asked))
			for _, q := range s.asked {
				if !q.Incoming {
					t.Fatalf("asked for outgoing messages too: %+v", q)
				}
				asked = append(asked, q.Chat.User+":"+strconv.Itoa(q.Limit))
			}
			if err != nil || len(got) != tt.total || !slices.Equal(asked, tt.want) {
				t.Fatalf("unread() = %d messages, %v; asked %v, want %v", len(got), err, asked, tt.want)
			}
		})
	}
}

func TestARepliedPollIsQuotedByItsName(t *testing.T) {
	t.Parallel()
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	poll := &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}}}}
	reply := &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("pizza it is"), ContextInfo: &wire.ContextInfo{
		StanzaId: new("3EB0POLL"), Participant: new(bob.String()), QuotedMessage: poll,
	}}}
	dir := directory{names: map[node.JID]store.Name{bob: {Contact: "Bob"}}}
	if got := describeMessage(dir, store.Message{ID: "3EB0", Chat: bob, Author: bob, Message: reply}); got.Text != "pizza it is" || got.Quote != `Bob: "Lunch?"` || got.ReplyTo != "3EB0POLL" {
		t.Fatalf("describeMessage() = %+v", got)
	}
}

func TestStatusShowsHistoryUntilItIsAllThere(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		history messenger.HistorySync
		shown   bool
	}{
		{history: messenger.HistorySync{}, shown: false},
		{history: messenger.HistorySync{Started: true, Percent: 0}, shown: true},
		{history: messenger.HistorySync{Started: true, Percent: 99}, shown: true},
		{history: messenger.HistorySync{Started: true, Percent: 100}, shown: false},
	} {
		_, detail := connection(messenger.Connection{Linked: true, Connected: true, History: tt.history}, "Linked.")
		if strings.Contains(detail, "History from the phone") != tt.shown {
			t.Errorf("%+v: %q", tt.history, detail)
		}
	}
}
