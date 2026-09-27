package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"modernc.org/sqlite"

	"bytes"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

var (
	bob    = node.JID{User: "40722222222", Server: node.ServerUser}
	carol  = node.JID{User: "40733333333", Server: node.ServerUser}
	family = node.JID{User: "120363000000000021", Server: node.ServerGroup}
	me     = node.JID{User: "40711111111", Server: node.ServerUser}
)

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

func open(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(ctx(t), filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func text(id string, chat, author node.JID, at int64, body string) store.Message {
	return store.Message{ID: id, Chat: chat, Author: author, FromMe: author == me, Time: time.Unix(at, 0), PushName: "P" + id, Message: &wire.Message{Conversation: new(body)}}
}

func rawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	connector, err := sqlite.NewConnector("file:" + (&url.URL{Path: path}).EscapedPath())
	if err != nil {
		t.Fatal(err)
	}
	return sql.OpenDB(connector)
}

func ids(messages []store.Message) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, m.ID)
	}
	return out
}

func TestMessagesComeBackInTimeOrderWhateverTheArrivalOrder(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("m3", bob, bob, 1790000300, "third"),
		text("m1", bob, bob, 1790000100, "first"),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("m2", bob, me, 1790000200, "second"),
		text("g1", family, carol, 1790000250, "in the group"),
	}}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10})
	if err != nil || !slices.Equal(ids(all), []string{"m1", "m2", "m3"}) {
		t.Fatalf("Messages(bob) = %v, %v", ids(all), err)
	}
	got := all[1]
	if got.Chat != bob || got.Author != me || !got.FromMe || !got.Time.Equal(time.Unix(1790000200, 0)) || got.PushName != "Pm2" || got.Message.GetConversation() != "second" {
		t.Fatalf("message = %+v", got)
	}
	latest, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 2})
	if err != nil || !slices.Equal(ids(latest), []string{"m2", "m3"}) {
		t.Fatalf("latest two = %v, %v", ids(latest), err)
	}
	older, err := s.Messages(ctx(t), store.Query{Chat: bob, Before: time.Unix(1790000200, 0), Limit: 10})
	if err != nil || !slices.Equal(ids(older), []string{"m1"}) {
		t.Fatalf("before the second = %v, %v", ids(older), err)
	}
	everywhere, err := s.Messages(ctx(t), store.Query{Limit: 10})
	if err != nil || !slices.Equal(ids(everywhere), []string{"m1", "m2", "g1", "m3"}) {
		t.Fatalf("all chats = %v, %v", ids(everywhere), err)
	}
	if none, err := s.Messages(ctx(t), store.Query{Chat: carol}); err != nil || len(none) != 0 {
		t.Fatalf("a chat without messages = %v, %v", ids(none), err)
	}
	if one, err := s.Messages(ctx(t), store.Query{Chat: bob}); err != nil || !slices.Equal(ids(one), []string{"m3"}) {
		t.Fatalf("a zero limit means one: %v, %v", ids(one), err)
	}
}

func TestTheSameMessageTwiceIsUpdatedNotDuplicated(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000100, "original words")}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000100, "edited words"), text("m1", carol, carol, 1790000100, "same id elsewhere")}}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Limit: 10})
	if err != nil || len(all) != 2 {
		t.Fatalf("messages = %v, %v", ids(all), err)
	}
	if found, err := s.Messages(ctx(t), store.Query{Text: "original", Limit: 10}); err != nil || len(found) != 0 {
		t.Fatalf("the replaced text is still searchable: %v, %v", ids(found), err)
	}
	if found, err := s.Messages(ctx(t), store.Query{Text: "edited", Limit: 10}); err != nil || len(found) != 1 || found[0].Message.GetConversation() != "edited words" {
		t.Fatalf("the new text is not searchable: %v, %v", ids(found), err)
	}
}

func TestMessagesOfOneSecondKeepTheirArrivalOrder(t *testing.T) {
	t.Parallel()
	s := open(t)
	batch := make([]store.Message, 0, 5000)
	for i := range 5000 {
		batch = append(batch, text(fmt.Sprintf("x%04d", i), bob, bob, 1790000000, fmt.Sprint("n", i)))
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: batch}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("later", bob, bob, 1790000001, "next second"), text("x5000", bob, bob, 1790000000, "late arrival")}}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10000})
	if err != nil || len(all) != 5002 {
		t.Fatalf("%d messages, %v", len(all), err)
	}
	for i, m := range all[:5001] {
		if want := fmt.Sprintf("x%04d", i); m.ID != want {
			t.Fatalf("message %d is %s, want %s: messages of one second must keep their arrival order", i, m.ID, want)
		}
	}
	if all[5001].ID != "later" {
		t.Fatalf("last = %s", all[5001].ID)
	}
}

func TestSearching(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("a", bob, bob, 1790000001, "Sănătate și bucurie"),
		text("b", bob, me, 1790000002, "ședința de mâine"),
		text("c", family, carol, 1790000003, "mâine la mare"),
		text("d", family, carol, 1790000004, `he said "hi" to everyone`),
		{ID: "e", Chat: bob, Author: bob, Time: time.Unix(1790000005, 0), Message: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("vacation photo")}}},
	}}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		query store.Query
		want  []string
	}{
		{name: "without diacritics", query: store.Query{Text: "sanatate"}, want: []string{"a"}},
		{name: "any case", query: store.Query{Text: "SEDINTA"}, want: []string{"b"}},
		{name: "prefix", query: store.Query{Text: "bucur"}, want: []string{"a"}},
		{name: "every word must match", query: store.Query{Text: "maine mare"}, want: []string{"c"}},
		{name: "word in two chats", query: store.Query{Text: "maine"}, want: []string{"b", "c"}},
		{name: "limited to a chat", query: store.Query{Text: "maine", Chat: family}, want: []string{"c"}},
		{name: "quotes are words", query: store.Query{Text: `"hi`}, want: []string{"d"}},
		{name: "operators are words", query: store.Query{Text: "hi OR NOT mare"}, want: nil},
		{name: "captions", query: store.Query{Text: "vacation"}, want: []string{"e"}},
		{name: "blank means everything", query: store.Query{Text: "  "}, want: []string{"a", "b", "c", "d", "e"}},
		{name: "latest first when limited", query: store.Query{Text: "maine", Limit: 1}, want: []string{"c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := tt.query
			if q.Limit == 0 {
				q.Limit = 10
			}
			got, err := s.Messages(ctx(t), q)
			if err != nil || !slices.Equal(ids(got), tt.want) && len(got)+len(tt.want) > 0 {
				t.Fatalf("Messages(%+v) = %v, %v; want %v", q, ids(got), err, tt.want)
			}
		})
	}
}

func TestFindingOneMessageByID(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("dup", bob, bob, 1790000001, "older"), text("dup", family, carol, 1790000009, "newer"), text("solo", bob, bob, 1790000002, "x")}}); err != nil {
		t.Fatal(err)
	}
	if m, ok, err := s.Message(ctx(t), "solo"); err != nil || !ok || m.Chat != bob {
		t.Fatalf("Message(solo) = %+v, %v, %v", m, ok, err)
	}
	if m, ok, err := s.Message(ctx(t), "dup"); err != nil || !ok || m.Message.GetConversation() != "newer" {
		t.Fatalf("an id in two chats gives the newest: %+v, %v, %v", m, ok, err)
	}
	if _, ok, err := s.Message(ctx(t), "nope"); err != nil || ok {
		t.Fatalf("Message(nope) = %v, %v", ok, err)
	}
}

func TestChatsAreListedByLatestActivity(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Chats: []store.Chat{
		{JID: family, Name: "Family", LastMessage: time.Unix(1790000500, 0)},
		{JID: carol, Name: "Carol", LastMessage: time.Unix(1790000100, 0)},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000900, "hi"), text("m0", family, carol, 1790000001, "old")}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Chats: []store.Chat{{JID: family, LastMessage: time.Unix(1790000002, 0)}, {JID: carol, Name: "Carol M", LastMessage: time.Unix(1790000200, 0)}}}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.Chats(ctx(t), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []store.Chat{
		{JID: bob, LastMessage: time.Unix(1790000900, 0)},
		{JID: family, Name: "Family", LastMessage: time.Unix(1790000500, 0)},
		{JID: carol, Name: "Carol M", LastMessage: time.Unix(1790000200, 0)},
	}
	if len(chats) != len(want) {
		t.Fatalf("chats = %+v", chats)
	}
	for i := range want {
		if chats[i].JID != want[i].JID || chats[i].Name != want[i].Name || !chats[i].LastMessage.Equal(want[i].LastMessage) {
			t.Errorf("chat %d = %+v, want %+v", i, chats[i], want[i])
		}
	}
	if first, err := s.Chats(ctx(t), 0); err != nil || len(first) != 1 || first[0].JID != bob {
		t.Fatalf("Chats(0) = %+v, %v", first, err)
	}
}

func TestNamesAndLIDsMerge(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Names: map[node.JID]store.Name{bob: {Push: "Bobby"}, carol: {Contact: "Carol Mihai", First: "Carol"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Names: map[node.JID]store.Name{bob: {Contact: "Bob Builder"}, carol: {Push: "Caro"}}}); err != nil {
		t.Fatal(err)
	}
	names, err := s.Names(ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	if names[bob] != (store.Name{Contact: "Bob Builder", Push: "Bobby"}) || names[carol] != (store.Name{Contact: "Carol Mihai", First: "Carol", Push: "Caro"}) || len(names) != 2 {
		t.Fatalf("names = %+v", names)
	}
	lid := node.JID{User: "88123", Server: node.ServerLID}
	if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{lid: carol}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{lid: bob}}); err != nil {
		t.Fatal(err)
	}
	if lids, err := s.LIDs(ctx(t)); err != nil || len(lids) != 1 || lids[lid] != bob {
		t.Fatalf("lids = %v, %v", lids, err)
	}
}

func TestReopeningKeepsEverythingAndRefusesANewerSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "a folder with spaces", "é #1?.db")
	if _, err := store.Open(ctx(t), path); err == nil {
		t.Fatal("opening inside a missing folder must fail")
	}
	path = filepath.Join(t.TempDir(), "é #1?.db")
	s, err := store.Open(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000100, "kept")}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := s.Messages(ctx(t), store.Query{Text: "kept", Limit: 1}); err != nil || len(found) != 1 {
		t.Fatalf("after reopening: %v, %v", ids(found), err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw := rawDB(t, path)
	if _, err := raw.ExecContext(ctx(t), `PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(ctx(t), path); !errors.Is(err, store.ErrNewer) {
		t.Fatalf("a newer schema: %v", err)
	}
}

func TestTimesOutsideTheKeyRange(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("past", bob, bob, -5, "before 1970"),
		text("future", bob, bob, 1<<50, "far future"),
		text("now", bob, bob, 1790000000, "now"),
	}}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10})
	if err != nil || !slices.Equal(ids(all), []string{"past", "now", "future"}) || all[0].Time.Unix() != 0 {
		t.Fatalf("messages = %v, %v", ids(all), err)
	}
}

func TestSearchableText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  *wire.Message
		want string
	}{
		{name: "nothing", msg: nil, want: ""},
		{name: "plain", msg: &wire.Message{Conversation: new("  hi  ")}, want: "hi"},
		{name: "extended", msg: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("see https://x.example")}}, want: "see https://x.example"},
		{name: "image caption", msg: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sunset")}}, want: "sunset"},
		{name: "video caption", msg: &wire.Message{VideoMessage: &wire.Message_VideoMessage{Caption: new("clip")}}, want: "clip"},
		{name: "document", msg: &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{FileName: new("cv.pdf"), Caption: new("my cv")}}, want: "cv.pdf\nmy cv"},
		{name: "contact", msg: &wire.Message{ContactMessage: &wire.Message_ContactMessage{DisplayName: new("Dan")}}, want: "Dan"},
		{name: "location", msg: &wire.Message{LocationMessage: &wire.Message_LocationMessage{Name: new("Home"), Address: new("Str. X 1")}}, want: "Home\nStr. X 1"},
		{name: "poll", msg: &wire.Message{PollCreationMessage: &wire.Message_PollCreationMessage{Name: new("Lunch?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}}}}, want: "Lunch?\nPizza\nSushi"},
		{name: "poll v3", msg: &wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Where?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Here")}}}}, want: "Where?\nHere"},
		{name: "contacts", msg: &wire.Message{ContactsArrayMessage: &wire.Message_ContactsArrayMessage{DisplayName: new("2 contacts"), Contacts: []*wire.Message_ContactMessage{{DisplayName: new("Dan")}, {DisplayName: new("Ana")}}}}, want: "2 contacts\nDan\nAna"},
		{name: "location comment", msg: &wire.Message{LocationMessage: &wire.Message_LocationMessage{Comment: new("gate B")}}, want: "gate B"},
		{name: "live location", msg: &wire.Message{LiveLocationMessage: &wire.Message_LiveLocationMessage{Caption: new("on my way")}}, want: "on my way"},
		{name: "sent from our phone", msg: &wire.Message{DeviceSentMessage: &wire.Message_DeviceSentMessage{Message: &wire.Message{Conversation: new("mine")}}}, want: "mine"},
		{name: "ephemeral", msg: &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{Conversation: new("gone soon")}}}, want: "gone soon"},
		{name: "view once", msg: &wire.Message{ViewOnceMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("once")}}}}, want: "once"},
		{name: "view once v2", msg: &wire.Message{ViewOnceMessageV2: &wire.Message_FutureProofMessage{Message: &wire.Message{Conversation: new("v2")}}}, want: "v2"},
		{name: "view once v2 extension", msg: &wire.Message{ViewOnceMessageV2Extension: &wire.Message_FutureProofMessage{Message: &wire.Message{Conversation: new("v2x")}}}, want: "v2x"},
		{name: "document with caption", msg: &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{FileName: new("a.txt")}}}}, want: "a.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := store.Text(tt.msg); got != tt.want {
				t.Fatalf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
	deep := &wire.Message{Conversation: new("bottom")}
	for range 8 {
		deep = &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: deep}}
	}
	if got := store.Text(deep); got != "bottom" {
		t.Fatalf("text under eight wrappers must be found like its media, got %q", got)
	}
	deep = &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: deep}}
	if got := store.Text(deep); got != "" {
		t.Fatalf("text under nine wrappers must not be reached, got %q", got)
	}
}

func TestAClosedOrBrokenStoreReportsErrors(t *testing.T) {
	t.Parallel()
	if _, err := store.Open(ctx(t), t.TempDir()); err == nil {
		t.Fatal("a folder opened as a store")
	}
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := store.Open(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("ok", bob, bob, 1790000001, "fine"), text("bad", bob, bob, 1790000002, "broken")}}); err != nil {
		t.Fatal(err)
	}
	raw := rawDB(t, path)
	if _, err := raw.ExecContext(ctx(t), `UPDATE messages SET raw = x'ffffff' WHERE id = 'bad'`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx(t), `INSERT INTO votes (chat, message_id, voter, options, t) VALUES (?, 'ok', ?, 'not json', 1)`, bob.String(), bob.String()); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10}); err == nil {
		t.Fatal("a corrupt stored message was read without an error")
	}
	if _, _, err := s.Message(ctx(t), "bad"); err == nil {
		t.Fatal("a corrupt stored message was found without an error")
	}
	if _, _, err := s.Message(ctx(t), "ok"); err == nil {
		t.Fatal("a corrupt stored vote was read without an error")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := ctx(t)
	for name, call := range map[string]func() error{
		"AddMessages": func() error {
			return s.Apply(ctx, store.Changes{Messages: []store.Message{text("x", bob, bob, 1, "x")}})
		},
		"AddChats": func() error { return s.Apply(ctx, store.Changes{Chats: []store.Chat{{JID: bob}}}) },
		"AddNames": func() error { return s.Apply(ctx, store.Changes{Names: map[node.JID]store.Name{bob: {Push: "b"}}}) },
		"AddLIDs":  func() error { return s.Apply(ctx, store.Changes{LIDs: map[node.JID]node.JID{bob: bob}}) },
		"AddVotes": func() error {
			return s.Apply(ctx, store.Changes{Votes: []store.Vote{{Chat: bob, ID: "ok", By: bob, Options: []string{"x"}}}})
		},
		"Messages": func() error { _, err := s.Messages(ctx, store.Query{Text: "x"}); return err },
		"Message":  func() error { _, _, err := s.Message(ctx, "ok"); return err },
		"Chats":    func() error { _, err := s.Chats(ctx, 1); return err },
		"Names":    func() error { _, err := s.Names(ctx); return err },
		"LIDs":     func() error { _, err := s.LIDs(ctx); return err },
	} {
		if err := call(); err == nil {
			t.Errorf("%s on a closed store succeeded", name)
		}
	}
}

func TestEditsDeletesAndReactions(t *testing.T) {
	t.Parallel()
	s := open(t)
	bobPhone := node.JID{User: bob.User, Device: 3, Server: bob.Server}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("m1", bob, bobPhone, 1790000100, "helo wrold"),
		text("m2", bob, bob, 1790000200, "secret plans"),
		text("m3", family, carol, 1790000300, "dinner?"),
	}}); err != nil {
		t.Fatal(err)
	}
	if m, ok, err := s.MessageIn(ctx(t), bob, "m1"); err != nil || !ok || m.Author != bob {
		t.Fatalf("a message sent from a companion device is kept under the account: %+v, %v, %v", m, ok, err)
	}
	if _, ok, err := s.MessageIn(ctx(t), family, "m1"); err != nil || ok {
		t.Fatalf("MessageIn of another chat = %v, %v", ok, err)
	}
	if err := s.Apply(ctx(t), store.Changes{
		Edits:   []store.Edit{{Chat: bob, ID: "m1", Message: &wire.Message{Conversation: new("hello world")}, Time: time.Unix(1790000150, 0)}},
		Revokes: []store.Revoke{{Chat: bob, ID: "m2"}},
		Reactions: []store.React{
			{Chat: family, ID: "m3", By: bobPhone, Emoji: "👍", Time: time.Unix(1790000301, 0)},
			{Chat: family, ID: "m3", By: me, Emoji: "❤️", Time: time.Unix(1790000302, 0)},
			{Chat: bob, ID: "m2", By: me, Emoji: "😮", Time: time.Unix(1790000201, 0)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for word, want := range map[string]int{"hello": 1, "helo": 0, "secret": 0} {
		if found, err := s.Messages(ctx(t), store.Query{Text: word, Limit: 5}); err != nil || len(found) != want {
			t.Errorf("search %q found %d, want %d: %v", word, len(found), want, err)
		}
	}
	edited, _, err := s.MessageIn(ctx(t), bob, "m1")
	if err != nil || edited.Message.GetConversation() != "hello world" || !edited.Edited.Equal(time.Unix(1790000150, 0)) {
		t.Fatalf("edited = %+v, %v", edited, err)
	}
	deleted, _, err := s.MessageIn(ctx(t), bob, "m2")
	if err != nil || !deleted.Revoked || store.Text(deleted.Message) != "" || len(deleted.Reactions) != 0 {
		t.Fatalf("deleted = %+v, %v", deleted, err)
	}
	if err := s.Apply(ctx(t), store.Changes{
		Messages: []store.Message{text("m1", bob, bob, 1790000100, "helo wrold"), text("m2", bob, bob, 1790000200, "secret plans")},
		Edits:    []store.Edit{{Chat: bob, ID: "m2", Message: &wire.Message{Conversation: new("back from the dead")}, Time: time.Unix(1790000400, 0)}},
		Reactions: []store.React{
			{Chat: family, ID: "m3", By: bob, Emoji: "😂", Time: time.Unix(1790000300, 0)},
			{Chat: family, ID: "m3", By: me, Emoji: "", Time: time.Unix(1790000303, 0)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Limit: 10})
	if err != nil || len(all) != 3 {
		t.Fatalf("messages = %v, %v", ids(all), err)
	}
	if all[0].Message.GetConversation() != "hello world" || !all[1].Revoked || store.Text(all[1].Message) != "" {
		t.Fatalf("a late copy undid an edit or a delete: %+v / %+v", all[0], all[1])
	}
	if r := all[2].Reactions; len(r) != 1 || r[0].By != bob || r[0].Emoji != "👍" {
		t.Fatalf("reactions after an older change and a removal = %+v", r)
	}
	if one, ok, err := s.Message(ctx(t), "m3"); err != nil || !ok || len(one.Reactions) != 1 {
		t.Fatalf("Message() carries reactions too: %+v, %v", one, err)
	}
}

func TestPollVotes(t *testing.T) {
	t.Parallel()
	s := open(t)
	bobPhone := node.JID{User: bob.User, Device: 3, Server: bob.Server}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("p1", family, carol, 1790000300, "Lunch?"),
		text("p2", bob, bob, 1790000200, "Movie?"),
	}}); err != nil {
		t.Fatal(err)
	}
	vote := func(chat node.JID, id string, by node.JID, at int64, options ...string) store.Vote {
		return store.Vote{Chat: chat, ID: id, By: by, Options: options, Time: time.Unix(at, 0)}
	}
	if err := s.Apply(ctx(t), store.Changes{
		Revokes: []store.Revoke{{Chat: bob, ID: "p2"}},
		Votes: []store.Vote{
			vote(family, "p1", bobPhone, 1790000301, "Pizza"),
			vote(family, "p1", me, 1790000302, "Pizza", "Sushi"),
			vote(family, "p1", carol, 1790000303, "Sushi"),
			vote(bob, "p2", me, 1790000201, "Yes"),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Votes: []store.Vote{
		vote(family, "p1", bob, 1790000300, "Sushi"),
		vote(family, "p1", me, 1790000304),
		vote(family, "p1", carol, 1790000303, "Pizza", "Soup"),
	}}); err != nil {
		t.Fatal(err)
	}
	poll, ok, err := s.MessageIn(ctx(t), family, "p1")
	if err != nil || !ok {
		t.Fatalf("poll = %v, %v", ok, err)
	}
	want := []store.Vote{vote(family, "p1", bob, 1790000301, "Pizza"), vote(family, "p1", carol, 1790000303, "Pizza", "Soup")}
	if len(poll.Votes) != len(want) {
		t.Fatalf("votes = %+v", poll.Votes)
	}
	for i, v := range poll.Votes {
		if v.Chat != want[i].Chat || v.ID != want[i].ID || v.By != want[i].By || !slices.Equal(v.Options, want[i].Options) || !v.Time.Equal(want[i].Time) {
			t.Fatalf("vote %d = %+v, want %+v", i, v, want[i])
		}
	}
	gone, _, err := s.MessageIn(ctx(t), bob, "p2")
	if err != nil || !gone.Revoked || len(gone.Votes) != 0 {
		t.Fatalf("a deleted poll kept votes: %+v, %v", gone, err)
	}
	if all, err := s.Messages(ctx(t), store.Query{Chat: family, Limit: 5}); err != nil || len(all) != 1 || len(all[0].Votes) != 2 {
		t.Fatalf("Messages() carries votes too: %+v, %v", all, err)
	}
}

func TestUnreadCountsAndTicks(t *testing.T) {
	t.Parallel()
	s := open(t)
	incoming := func(id string, at int64) store.Message {
		m := text(id, bob, bob, at, "hi "+id)
		m.Unread = true
		return m
	}
	unreadOf := func(chat node.JID) int {
		t.Helper()
		chats, err := s.Chats(ctx(t), 10)
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
	if err := s.Apply(ctx(t), store.Changes{Unread: map[node.JID]int{family: 4, carol: -2}}); err != nil {
		t.Fatal(err)
	}
	if got := unreadOf(family); got != 4 {
		t.Fatalf("family unread = %d", got)
	}
	if got := unreadOf(carol); got != 0 {
		t.Fatalf("a negative count must be stored as zero, got %d", got)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{incoming("i1", 1790000001), incoming("i2", 1790000002), text("o1", bob, me, 1790000003, "mine")}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{incoming("i1", 1790000001)}}); err != nil {
		t.Fatal(err)
	}
	if got := unreadOf(bob); got != 2 {
		t.Fatalf("bob unread = %d, want two new incoming messages counted once", got)
	}
	if err := s.Apply(ctx(t), store.Changes{Seen: []node.JID{bob}}); err != nil {
		t.Fatal(err)
	}
	if got, fam := unreadOf(bob), unreadOf(family); got != 0 || fam != 4 {
		t.Fatalf("after seeing bob's chat: bob %d, family %d", got, fam)
	}
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("o2", bob, me, 1790000004, "mine too")}, Ticks: []store.Tick{
		{Chat: bob, IDs: []string{"o1", "i1"}, Status: store.StatusRead},
		{Chat: bob, IDs: []string{"o1"}, Status: store.StatusDelivered},
		{Chat: bob, IDs: []string{"o2"}, Status: store.StatusDelivered},
		{Chat: family, IDs: []string{"o2"}, Status: store.StatusPlayed},
	}}); err != nil {
		t.Fatal(err)
	}
	all, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]store.Status{}
	for _, m := range all {
		status[m.ID] = m.Status
	}
	if status["o1"] != store.StatusRead || status["o2"] != store.StatusDelivered || status["i1"] != store.StatusSent {
		t.Fatalf("statuses = %v: ticks only move forward, only on our messages, only in their chat", status)
	}
}

func TestStatusUpdatesStayOutOfChatsAndAllChatReads(t *testing.T) {
	t.Parallel()
	s := open(t)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("s1", node.StatusBroadcast(), bob, 1790000200, "at the beach"),
		text("m1", bob, bob, 1790000100, "hi"),
	}}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.Chats(ctx(t), 10)
	if err != nil || len(chats) != 1 || chats[0].JID != bob {
		t.Fatalf("chats = %+v, %v", chats, err)
	}
	if all, err := s.Messages(ctx(t), store.Query{Limit: 10}); err != nil || !slices.Equal(ids(all), []string{"m1"}) {
		t.Fatalf("all chats = %v, %v", ids(all), err)
	}
	if found, err := s.Messages(ctx(t), store.Query{Text: "beach", Limit: 10}); err != nil || len(found) != 0 {
		t.Fatalf("searching all chats found a status: %v, %v", ids(found), err)
	}
	if statuses, err := s.Messages(ctx(t), store.Query{Chat: node.StatusBroadcast(), Limit: 10}); err != nil || !slices.Equal(ids(statuses), []string{"s1"}) {
		t.Fatalf("status updates = %v, %v", ids(statuses), err)
	}
}

func TestChatsFollowThePhone(t *testing.T) {
	t.Parallel()
	s := open(t)
	dan := node.JID{User: "40744444444", Server: node.ServerUser}
	eve := node.JID{User: "40766666666", Server: node.ServerUser}
	if err := s.Apply(ctx(t), store.Changes{
		Chats:    []store.Chat{{JID: family, Name: "Family", LastMessage: time.Unix(1790000500, 0)}},
		Messages: []store.Message{text("b1", bob, bob, 1790000900, "hi"), text("c1", carol, carol, 1790000800, "yo"), text("d1", dan, dan, 1790000950, "hey")},
		Unread:   map[node.JID]int{bob: 2},
	}); err != nil {
		t.Fatal(err)
	}
	save := func(changes store.SyncChanges) {
		t.Helper()
		if err := s.SaveSync(ctx(t), "regular_low", store.SyncState{Version: 1, Hash: make([]byte, 128)}, changes); err != nil {
			t.Fatal(err)
		}
	}
	save(store.SyncChanges{
		Pins:     map[node.JID]time.Time{family: time.UnixMilli(1790000001000), carol: time.UnixMilli(1790000002000)},
		Archives: map[node.JID]bool{dan: true, bob: true, eve: true},
		Mutes:    map[node.JID]store.Mute{family: store.MuteForever, bob: 1790003600},
	})
	save(store.SyncChanges{Pins: map[node.JID]time.Time{bob: time.UnixMilli(1790000003000), carol: {}}, Archives: map[node.JID]bool{dan: false}})
	chats, err := s.Chats(ctx(t), 10)
	if err != nil {
		t.Fatal(err)
	}
	order := make([]string, 0, len(chats))
	for _, c := range chats {
		order = append(order, c.JID.User)
	}
	want := []string{bob.User, family.User, dan.User, carol.User, eve.User}
	if !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	b, f, d, c, e := chats[0], chats[1], chats[2], chats[3], chats[4]
	if !b.Pinned.Equal(time.UnixMilli(1790000003000)) || b.Archived || b.Unread != 2 || b.Mute != 1790003600 || b.LastMessage.Unix() != 1790000900 {
		t.Fatalf("pinning unarchives and keeps the rest: %+v", b)
	}
	if f.Name != "Family" || f.Mute != store.MuteForever || f.Archived {
		t.Fatalf("family = %+v", f)
	}
	if d.Archived || !d.Pinned.IsZero() || c.Archived || !c.Pinned.IsZero() {
		t.Fatalf("unarchived and unpinned: %+v / %+v", d, c)
	}
	if !e.Archived || e.Name != "" || !e.LastMessage.Equal(time.Unix(0, 0)) {
		t.Fatalf("a chat known only from its flags = %+v", e)
	}
}

func TestMuteIsActive(t *testing.T) {
	t.Parallel()
	now := time.Unix(1790000000, 0)
	for _, tt := range []struct {
		mute store.Mute
		want bool
	}{
		{store.MuteForever, true},
		{0, false},
		{1790000001, true},
		{1790000000, false},
		{-2, false},
	} {
		if got := tt.mute.Active(now); got != tt.want {
			t.Errorf("Mute(%d).Active() = %v", tt.mute, got)
		}
	}
}

func TestIncomingOnly(t *testing.T) {
	t.Parallel()
	s := open(t)
	mine := text("m2", bob, me, 1790000200, "mine")
	mine.FromMe = true
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000100, "theirs"), mine, text("m3", bob, bob, 1790000300, "theirs again")}}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 2, Incoming: true}); err != nil || !slices.Equal(ids(got), []string{"m1", "m3"}) {
		t.Fatalf("incoming = %v, %v", ids(got), err)
	}
}

func TestTheStoreIsPrivate(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps a user's files private through the profile folder's ACL, not mode bits")
	}
	dir := t.TempDir()
	older := filepath.Join(dir, "older.db")
	if err := os.WriteFile(older, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "new.db"), older} {
		s, err := store.Open(ctx(t), path)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000000, "private")}}); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{path, path + "-wal", path + "-shm"} {
			if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
				t.Errorf("%s: %v, %v", filepath.Base(file), info.Mode(), err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestManyProcessesOpenOneNewStore(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shared.db")
	errs := make(chan error, 8)
	for range cap(errs) {
		go func() {
			s, err := store.Open(ctx(t), path)
			if err == nil {
				err = s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("m1", bob, bob, 1790000000, "hi")}})
				err = errors.Join(err, s.Close())
			}
			errs <- err
		}()
	}
	for range cap(errs) {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	s, err := store.Open(ctx(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if all, err := s.Messages(ctx(t), store.Query{Limit: 10}); err != nil || len(all) != 1 {
		t.Fatalf("messages = %d, %v", len(all), err)
	}
}

func TestPaceCountsWhatWeSent(t *testing.T) {
	t.Parallel()
	s := open(t)
	now := time.Unix(1790100000, 0)
	chat := func(user string) node.JID { return node.JID{User: user, Server: node.ServerUser} }
	ours := func(id string, to node.JID, ago time.Duration) store.Message {
		m := text(id, to, me, now.Add(-ago).Unix(), "hi")
		m.FromMe = true
		return m
	}
	a, b, c, d, e := chat("40711000001"), chat("40711000002"), chat("40711000003"), chat("40711000004"), chat("40711000005")
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		ours("a1", a, 2*time.Minute), ours("a2", a, 30*time.Second), ours("a3", a, 20*time.Second), ours("a4", a, 10*time.Second),
		ours("b1", b, time.Hour),
		text("c1", c, c, now.Add(-2*time.Hour).Unix(), "hello"), ours("c2", c, time.Hour),
		ours("d1", d, 25*time.Hour),
		ours("x1", family, time.Minute+time.Second),
		ours("l1", node.JID{User: "100000000000077", Server: node.ServerLID}, 3*time.Hour),
	}}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		chat node.JID
		want store.Pace
	}{
		{chat: a, want: store.Pace{LastMinute: 3, NewChats: 3, Known: true}},
		{chat: e, want: store.Pace{LastMinute: 3, NewChats: 3, Known: false}},
	} {
		if got, err := s.Pace(ctx(t), tt.chat, now); err != nil || got != tt.want {
			t.Errorf("Pace(%s) = %+v, %v; want %+v", tt.chat, got, err, tt.want)
		}
	}
}

func TestMessagesFromSomeone(t *testing.T) {
	t.Parallel()
	s := open(t)
	bobLID := node.JID{User: "100000000000002", Server: node.ServerLID}
	mine := text("f4", family, me, 1790000400, "me too")
	mine.FromMe = true
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		text("f1", family, bob, 1790000100, "trip on friday"),
		text("f2", family, node.JID{User: bobLID.User, Server: bobLID.Server, Device: 2}, 1790000200, "trip tickets bought"),
		text("f3", family, carol, 1790000300, "trip sounds good"),
		mine,
		text("b1", bob, bob, 1790000500, "trip question"),
	}}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		q    store.Query
		want []string
	}{
		{name: "one person, both addresses", q: store.Query{From: []node.JID{bob, bobLID}, Limit: 10}, want: []string{"f1", "f2", "b1"}},
		{name: "one person in one chat", q: store.Query{Chat: family, From: []node.JID{bob, bobLID}, Limit: 10}, want: []string{"f1", "f2"}},
		{name: "one person and words", q: store.Query{Text: "tickets", From: []node.JID{bob, bobLID}, Limit: 10}, want: []string{"f2"}},
		{name: "only the lid", q: store.Query{From: []node.JID{bobLID}, Limit: 10}, want: []string{"f2"}},
		{name: "mine", q: store.Query{Chat: family, Mine: true, Limit: 10}, want: []string{"f4"}},
		{name: "someone else", q: store.Query{Text: "trip", From: []node.JID{carol}, Limit: 10}, want: []string{"f3"}},
	} {
		if got, err := s.Messages(ctx(t), tt.q); err != nil || !slices.Equal(ids(got), tt.want) {
			t.Errorf("%s: %v, %v; want %v", tt.name, ids(got), err, tt.want)
		}
	}
}

func TestOneChatPerPersonWhateverTheirID(t *testing.T) {
	t.Parallel()
	s := open(t)
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	bobPhone := node.JID{User: "99001", Device: 3, Server: node.ServerLID}
	if err := s.Apply(ctx(t), store.Changes{
		Messages: []store.Message{
			text("h1", bob, bob, 1790000100, "from history"), text("dup", bob, bob, 1790000150, "seen twice"),
			text("l1", bobLID, bobPhone, 1790000200, "live hello"), text("dup", bobLID, bobLID, 1790000150, "seen twice"),
		},
		Reactions: []store.React{{Chat: bobLID, ID: "h1", By: me, Emoji: "👍", Time: time.Unix(1790000210, 0)}},
		Unread:    map[node.JID]int{bobLID: 2},
		Names:     map[node.JID]store.Name{bobLID: {Push: "Bobby"}, bob: {Contact: "Bob Builder"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSync(ctx(t), "regular_low", store.SyncState{Hash: []byte{1}}, store.SyncChanges{Pins: map[node.JID]time.Time{bobLID: time.Unix(1790000300, 0)}, Mutes: map[node.JID]store.Mute{bob: store.MuteForever}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{bobLID: bob}}); err != nil {
		t.Fatal(err)
	}
	chats, err := s.Chats(ctx(t), 10)
	if err != nil || len(chats) != 1 || chats[0].JID != bob || chats[0].Unread != 2 || chats[0].Pinned.IsZero() || chats[0].Mute != store.MuteForever {
		t.Fatalf("chats after learning the pairing = %+v, %v", chats, err)
	}
	for _, chat := range []node.JID{bob, bobLID, bobPhone} {
		got, err := s.Messages(ctx(t), store.Query{Chat: chat, Limit: 10})
		if err != nil || !slices.Equal(ids(got), []string{"h1", "dup", "l1"}) {
			t.Fatalf("reading %s = %v, %v", chat, ids(got), err)
		}
		if len(got[0].Reactions) != 1 || got[0].Reactions[0].Emoji != "👍" {
			t.Fatalf("reactions on h1 = %+v", got[0].Reactions)
		}
	}
	if found, err := s.Messages(ctx(t), store.Query{Text: "seen twice", Limit: 10}); err != nil || !slices.Equal(ids(found), []string{"dup"}) {
		t.Fatalf("search after the fold = %v, %v", ids(found), err)
	}
	names, err := s.Names(ctx(t))
	if err != nil || names[bob] != (store.Name{Contact: "Bob Builder", Push: "Bobby"}) || len(names) != 1 {
		t.Fatalf("names = %+v, %v", names, err)
	}

	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{text("l2", bobLID, bobPhone, 1790000400, "later"), text("s1", bob, me, 1790000410, "mine")}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx(t), store.Changes{
		Ticks:   []store.Tick{{Chat: bobPhone, IDs: []string{"s1"}, Status: store.StatusRead}},
		Edits:   []store.Edit{{Chat: bobLID, ID: "l2", Message: &wire.Message{Conversation: new("later, edited")}, Time: time.Unix(1790000420, 0)}},
		Revokes: []store.Revoke{{Chat: bobPhone, ID: "l1"}},
		Votes:   []store.Vote{{Chat: bobLID, ID: "h1", By: bobPhone, Options: []string{"yes"}, Time: time.Unix(1790000430, 0)}},
		Seen:    []node.JID{bobLID},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Messages(ctx(t), store.Query{Chat: bob, Limit: 10})
	if err != nil || !slices.Equal(ids(got), []string{"h1", "dup", "l1", "l2", "s1"}) {
		t.Fatalf("after live traffic = %v, %v", ids(got), err)
	}
	if !got[2].Revoked || got[3].Edited.IsZero() || store.Text(got[3].Message) != "later, edited" || got[4].Status != store.StatusRead || got[3].Author != bob {
		t.Fatalf("live changes by private id missed the chat: %+v", got)
	}
	if len(got[0].Votes) != 1 || got[0].Votes[0].By != bob {
		t.Fatalf("votes on h1 = %+v", got[0].Votes)
	}
	if found, ok, err := s.MessageIn(ctx(t), bobLID, "l2"); err != nil || !ok || found.Chat != bob {
		t.Fatalf("MessageIn by private id = %+v, %v, %v", found, ok, err)
	}
	if chats, err := s.Chats(ctx(t), 10); err != nil || len(chats) != 1 || chats[0].Unread != 0 {
		t.Fatalf("chats after live traffic = %+v, %v", chats, err)
	}
}

func TestPrivacyTokensKeepTheNewestAndFollowThePrivateID(t *testing.T) {
	s := open(t)
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	at := func(sec int64) time.Time { return time.Unix(sec, 0) }
	apply := func(tokens ...privacy.Token) {
		t.Helper()
		if err := s.Apply(ctx(t), store.Changes{Tokens: tokens}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(who node.JID, theirs []byte, given, ours time.Time) {
		t.Helper()
		got, err := s.Token(ctx(t), who)
		if err != nil || !bytes.Equal(got.Theirs, theirs) || !got.Given.Equal(given) || !got.Ours.Equal(ours) {
			t.Fatalf("Token(%s) = %+v, %v; want %v given %v, ours %v", who, got, err, theirs, given, ours)
		}
	}
	check(bob, nil, time.Time{}, time.Time{})

	apply(privacy.Token{Contact: bob, Theirs: []byte{2}, Given: at(2000)})
	apply(privacy.Token{Contact: bob, Theirs: []byte{1}, Given: at(1000)})
	apply(privacy.Token{Contact: bob, Ours: at(5000)})
	apply(privacy.Token{Contact: bob, Ours: at(4000)})
	check(bob, []byte{2}, at(2000), at(5000))

	apply(privacy.Token{Contact: bobLID, Theirs: []byte{3}, Given: at(3000), Ours: at(6000)})
	check(bobLID, []byte{3}, at(3000), at(6000))
	if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{bobLID: bob}}); err != nil {
		t.Fatal(err)
	}
	check(bob, []byte{3}, at(3000), at(6000))
	check(bobLID, []byte{3}, at(3000), at(6000))
}

func TestLearningAPairRewritesOldAuthorsOnce(t *testing.T) {
	s := open(t)
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	family := node.JID{User: "120363000000000031", Server: node.ServerGroup}
	at := time.Unix(1790000000, 0)
	if err := s.Apply(ctx(t), store.Changes{Messages: []store.Message{
		{ID: "G1", Chat: family, Author: bobLID, Time: at, Message: &wire.Message{Conversation: new("from the private id")}},
	}}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{bobLID: bob}}); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := s.MessageIn(ctx(t), family, "G1")
	if err != nil || !ok || got.Author != bob {
		t.Fatalf("after learning the pair the author is %v (%v, %v)", got.Author, ok, err)
	}
	for _, j := range []node.JID{bob, bobLID, {User: bob.User, Device: 3, Server: bob.Server}} {
		forms, err := s.Forms(ctx(t), j)
		if err != nil || len(forms) != 2 || !slices.Contains(forms, bob) || !slices.Contains(forms, bobLID) {
			t.Fatalf("Forms(%v) = %v, %v", j, forms, err)
		}
	}
	if c, err := s.Canonical(ctx(t), bobLID); err != nil || c != bob {
		t.Fatalf("Canonical(%v) = %v, %v", bobLID, c, err)
	}
	stranger := node.JID{User: "40799999999", Server: node.ServerUser}
	if forms, err := s.Forms(ctx(t), stranger); err != nil || len(forms) != 1 || forms[0] != stranger {
		t.Fatalf("Forms of someone without a pair = %v, %v", forms, err)
	}
}

func TestDisappearingTimersKeepTheNewestSetting(t *testing.T) {
	s := open(t)
	lid := node.JID{User: "98765", Server: node.ServerLID}
	week, day := uint32(7*24*3600), uint32(24*3600)
	steps := []struct {
		timer store.Timer
		want  uint32
	}{
		{timer: store.Timer{Chat: bob, Seconds: week, Set: time.Unix(2000, 0)}, want: week},
		{timer: store.Timer{Chat: bob, Seconds: day, Set: time.Unix(1000, 0)}, want: week},
		{timer: store.Timer{Chat: lid, Seconds: day, Set: time.Unix(3000, 0)}, want: day},
		{timer: store.Timer{Chat: bob, Seconds: 0, Set: time.Unix(4000, 0)}, want: 0},
	}
	if err := s.Apply(ctx(t), store.Changes{LIDs: map[node.JID]node.JID{lid: bob}}); err != nil {
		t.Fatal(err)
	}
	for i, step := range steps {
		if err := s.Apply(ctx(t), store.Changes{Timers: []store.Timer{step.timer}}); err != nil {
			t.Fatal(err)
		}
		got, err := s.Timer(ctx(t), lid)
		if err != nil || got.Seconds != step.want || got.Chat != bob {
			t.Fatalf("step %d: Timer() = %+v, %v; want %d seconds on %v", i, got, err, step.want, bob)
		}
	}
	if none, err := s.Timer(ctx(t), carol); err != nil || none.Seconds != 0 || !none.Set.IsZero() {
		t.Fatalf("a chat never seen = %+v, %v", none, err)
	}
}
