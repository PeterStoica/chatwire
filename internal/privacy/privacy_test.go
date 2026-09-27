package privacy_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
)

const week = 604800

func TestTheirTokenLastsFourWeeklyBuckets(t *testing.T) {
	now := time.Unix(2900*week+1000, 0)
	oldest := time.Unix(2897*week, 0)
	for _, tt := range []struct {
		name  string
		token privacy.Token
		want  bool
	}{
		{name: "given this week", token: privacy.Token{Theirs: []byte{1}, Given: now.Add(-time.Hour)}, want: true},
		{name: "given at the start of the oldest bucket", token: privacy.Token{Theirs: []byte{1}, Given: oldest}, want: true},
		{name: "given a second before it", token: privacy.Token{Theirs: []byte{1}, Given: oldest.Add(-time.Second)}},
		{name: "no token bytes", token: privacy.Token{Given: now}},
		{name: "no time", token: privacy.Token{Theirs: []byte{1}}},
	} {
		if got := tt.token.Usable(now); got != tt.want {
			t.Errorf("%s: Usable = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOursIsGivenOncePerWeeklyBucket(t *testing.T) {
	now := time.Unix(2900*week+1000, 0)
	for _, tt := range []struct {
		name string
		ours time.Time
		want bool
	}{
		{name: "never given", want: true},
		{name: "given earlier this bucket", ours: time.Unix(2900*week, 0)},
		{name: "given in the last second of the previous bucket", ours: time.Unix(2900*week-1, 0), want: true},
	} {
		if got := (privacy.Token{Ours: tt.ours}).Due(now); got != tt.want {
			t.Errorf("%s: Due = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestGivingOurToken(t *testing.T) {
	bob := node.JID{User: "40722222222", Device: 3, Server: node.ServerUser}
	got := privacy.Give(bob, time.Unix(1790000000, 0))
	want := "iq to=s.whatsapp.net type=set xmlns=privacy > tokens > token jid=40722222222@s.whatsapp.net t=1790000000 type=trusted_contact"
	if render(got) != want {
		t.Fatalf("Give\n got %s\nwant %s", render(got), want)
	}
}

func render(n node.Node) string {
	out := n.Tag
	for _, a := range n.Attrs {
		if !a.Value.IsZero() {
			out += " " + a.Key + "=" + a.Value.String()
		}
	}
	for _, c := range n.Children {
		out += " > " + render(c)
	}
	return out
}

func TestTokensTheyGaveUs(t *testing.T) {
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	mine := func(j node.JID) bool { return j.User == me.User }
	token := func(holder string, kind, stamp string, bytes []byte) node.Node {
		attrs := []node.Attr{{Key: "type", Value: node.Text(kind)}, {Key: "t", Value: node.Text(stamp)}}
		if holder != "" {
			j, err := node.ParseJID(holder)
			if err != nil {
				t.Fatal(err)
			}
			attrs = append(attrs, node.Attr{Key: "jid", Value: node.Address(j)})
		}
		return node.Node{Tag: "token", Attrs: attrs, Bytes: bytes}
	}
	notice := func(attrs []node.Attr, tokens ...node.Node) node.Node {
		return node.Node{Tag: "notification", Attrs: append([]node.Attr{{Key: "type", Value: node.Text("privacy_token")}, {Key: "id", Value: node.Text("N1")}}, attrs...),
			Children: []node.Node{{Tag: "tokens", Children: tokens}}}
	}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	from := node.Attr{Key: "from", Value: node.Address(bob)}
	got, err := privacy.Received(mine, notice([]node.Attr{from},
		token("40711111111@s.whatsapp.net", "trusted_contact", "1790000000", []byte{7, 7}),
		token("", "trusted_contact", "1790000500", []byte{8}),
		token("40799999999@s.whatsapp.net", "trusted_contact", "1790000000", []byte{9}),
		token("40711111111@s.whatsapp.net", "other", "1790000000", []byte{9}),
		token("40711111111@s.whatsapp.net", "trusted_contact", "soon", []byte{9}),
		token("40711111111@s.whatsapp.net", "trusted_contact", "1790000000", nil),
	))
	want := []privacy.Token{
		{Contact: bob, Theirs: []byte{7, 7}, Given: time.Unix(1790000000, 0)},
		{Contact: bob, Theirs: []byte{8}, Given: time.Unix(1790000500, 0)},
	}
	if err != nil || !slices.EqualFunc(got, want, same) {
		t.Fatalf("Received = %+v, %v", got, err)
	}
	got, err = privacy.Received(mine, notice([]node.Attr{from, {Key: "sender_lid", Value: node.Address(node.JID{User: "99001", Server: node.ServerLID})}},
		token("40711111111@s.whatsapp.net", "trusted_contact", "1790000000", []byte{1})))
	if err != nil || len(got) != 1 || got[0].Contact != (node.JID{User: "99001", Server: node.ServerLID}) {
		t.Fatalf("a token with a sender LID: %+v, %v", got, err)
	}
	if _, err := privacy.Received(mine, node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("server_sync")}}}); !errors.Is(err, privacy.ErrNotification) {
		t.Fatalf("another notification: %v", err)
	}
}

func same(a, b privacy.Token) bool {
	return a.Contact == b.Contact && slices.Equal(a.Theirs, b.Theirs) && a.Given.Equal(b.Given) && a.Ours.Equal(b.Ours)
}
