package mcptools

import (
	"context"
	"fmt"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

type bookSender struct {
	Sender
	chats []store.Chat
	names map[node.JID]store.Name
	lids  map[node.JID]node.JID
}

func (b bookSender) Self() (node.JID, bool) {
	return node.JID{User: "40711111111", Server: node.ServerUser}, true
}

func (b bookSender) Chats(context.Context, int) ([]store.Chat, error) {
	return append([]store.Chat(nil), b.chats...), nil
}

func (b bookSender) Names(context.Context) (map[node.JID]store.Name, error) {
	out := make(map[node.JID]store.Name, len(b.names))
	for k, v := range b.names {
		out[k] = v
	}
	return out, nil
}

func (b bookSender) LIDs(context.Context) (map[node.JID]node.JID, error) {
	return b.lids, nil
}

func addressBook(chats, contacts int) bookSender {
	b := bookSender{names: map[node.JID]store.Name{}, lids: map[node.JID]node.JID{}}
	for i := range contacts {
		pn := node.JID{User: fmt.Sprintf("4072%07d", i), Server: node.ServerUser}
		b.names[pn] = store.Name{Contact: fmt.Sprintf("Ștefan Popescu %d", i), First: "Ștefan", Push: fmt.Sprintf("Ștefi %d", i)}
		b.lids[node.JID{User: fmt.Sprintf("9%07d", i), Server: node.ServerLID}] = pn
		if i < chats {
			b.chats = append(b.chats, store.Chat{JID: pn, Name: fmt.Sprintf("Ștefan Popescu %d", i)})
		}
	}
	return b
}

func BenchmarkFindingSomeoneByName(b *testing.B) {
	book := addressBook(500, 3000)
	for b.Loop() {
		dir, err := loadDirectory(b.Context(), book)
		if err != nil {
			b.Fatal(err)
		}
		if found := dir.find("stefan popescu 2999"); len(found) != 1 {
			b.Fatalf("found %d", len(found))
		}
	}
}
