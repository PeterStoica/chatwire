package mcptools

import (
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

func TestFindingAPersonAmongChatsThatMentionThem(t *testing.T) {
	t.Parallel()
	user := func(n string) node.JID { return node.JID{User: n, Server: node.ServerUser} }
	group := func(n string) node.JID { return node.JID{User: n, Server: node.ServerGroup} }
	maria, ana, ana2 := user("40700000100"), user("40700000101"), user("40700000102")
	dir := directory{
		chats: []store.Chat{
			{JID: maria, Name: "Maria❤️❤️❤️"}, {JID: group("1"), Name: "‎‪Ion & Maria❤️❤️❤️"}, {JID: group("2"), Name: "Runners Dan&Maria"},
			{JID: group("3"), Name: "‎‪Maria❤️❤️❤️ & Radu US"}, {JID: group("4"), Name: "Maria"}, {JID: ana, Name: "Ana 🌸"}, {JID: ana2, Name: "ana!"},
		},
		names: map[node.JID]store.Name{},
		named: map[node.JID]string{},
	}
	for _, tt := range []struct {
		query string
		want  []node.JID
	}{
		{query: "maria❤️❤️❤️", want: []node.JID{maria}},
		{query: "Maria", want: []node.JID{group("4")}},
		{query: "Ana", want: []node.JID{ana, ana2}},
		{query: "radu", want: []node.JID{group("3")}},
		{query: "❤️", want: []node.JID{maria, group("1"), group("3")}},
	} {
		if got := dir.find(tt.query); !slices.Equal(got, tt.want) {
			t.Errorf("find(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
	dir.chats = slices.DeleteFunc(dir.chats, func(c store.Chat) bool { return c.JID == group("4") })
	if got := dir.find("Maria"); !slices.Equal(got, []node.JID{maria}) {
		t.Errorf("find(Maria) without a group of that exact name = %v, want her own chat", got)
	}
	if got := clean("‎‪Ion & Maria❤️❤️❤️‬"); got != "Ion & Maria❤️❤️❤️" {
		t.Errorf("clean = %q", got)
	}
}
