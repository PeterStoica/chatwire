package mcptools

import (
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

func TestMentioningGroupMembersByName(t *testing.T) {
	t.Parallel()
	user := func(n string) node.JID { return node.JID{User: n, Server: node.ServerUser} }
	lid := func(n string) node.JID { return node.JID{User: n, Server: node.ServerLID} }
	me, bob, mama, anaPop, anaIon, stefan := user("40711111111"), user("40722222222"), user("40733333333"), user("40744444444"), user("40755555555"), user("40766666666")
	carolLID, carol := lid("100000000000077"), user("40777777777")
	dir := directory{
		self: me,
		names: map[node.JID]store.Name{
			bob: {Contact: "Bob Smith", First: "Bob"}, mama: {Contact: "Mama Ioana Maria", First: "Mama"},
			anaPop: {Contact: "Ana Pop"}, anaIon: {Contact: "Ana Ionescu"}, stefan: {Push: "Ștefan"}, carol: {Contact: "Carol"},
		},
		lids: map[node.JID]node.JID{carolLID: carol},
	}
	people := []groups.Participant{{JID: me}, {JID: bob, LID: lid("100000000000022")}, {JID: mama}, {JID: anaPop}, {JID: anaIon}, {JID: stefan}, {LID: carolLID}}
	family := groups.Group{Participants: people}
	tests := []struct {
		name     string
		group    groups.Group
		text     string
		want     string
		mentions []node.JID
	}{
		{name: "a first name", group: family, text: "@Bob can you?", want: "@40722222222 can you?", mentions: []node.JID{bob}},
		{name: "a full name in any case", group: family, text: "thanks @bob smith!", want: "thanks @40722222222!", mentions: []node.JID{bob}},
		{name: "three words", group: family, text: "@Mama Ioana Maria are you there", want: "@40733333333 are you there", mentions: []node.JID{mama}},
		{name: "accents do not matter", group: family, text: "@stefan hi", want: "@40766666666 hi", mentions: []node.JID{stefan}},
		{name: "two people share a first name", group: family, text: "@Ana hi", want: "@Ana hi"},
		{name: "a full name settles it", group: family, text: "@Ana Pop hi", want: "@40744444444 hi", mentions: []node.JID{anaPop}},
		{name: "a number", group: family, text: "@+40722222222 and @40744444444", want: "@40722222222 and @40744444444", mentions: []node.JID{bob, anaPop}},
		{name: "a number not in the group", group: family, text: "@40799999999 hi", want: "@40799999999 hi"},
		{name: "twice the same person", group: family, text: "@Bob, @Bob!", want: "@40722222222, @40722222222!", mentions: []node.JID{bob}},
		{name: "an e-mail address", group: family, text: "write to bob@example.com", want: "write to bob@example.com"},
		{name: "a name that is not in the group", group: family, text: "@Zed hi", want: "@Zed hi"},
		{name: "an at sign alone", group: family, text: "meet @ 5 @", want: "meet @ 5 @"},
		{name: "a member known only by lid", group: family, text: "@Carol hi", want: "@100000000000077 hi", mentions: []node.JID{carolLID}},
		{name: "a lid group mentions lids", group: groups.Group{AddressingMode: "lid", Participants: people}, text: "@Bob hi", want: "@100000000000022 hi", mentions: []node.JID{lid("100000000000022")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, mentions := mentionsIn(dir, tt.group, tt.text)
			if got != tt.want || !slices.Equal(mentions, tt.mentions) {
				t.Fatalf("mentionsIn() = %q %v, want %q %v", got, mentions, tt.want, tt.mentions)
			}
		})
	}
}
