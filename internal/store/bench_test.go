package store_test

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

func benchmarkStore(b *testing.B, n int) *store.Store {
	b.Helper()
	s, err := store.Open(b.Context(), filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = s.Close() })
	words := strings.Fields("mâine sănătate ședință întâlnire când ce faci bună ziua mulțumesc factura plata meeting tomorrow invoice dinner flight hotel proiect vineri")
	r := rand.New(rand.NewPCG(1, 2))
	chats := make([]node.JID, 300)
	for i := range chats {
		chats[i] = node.JID{User: fmt.Sprintf("4072%07d", i), Server: node.ServerUser}
	}
	batch := make([]store.Message, 0, 1000)
	for i := range n {
		text := make([]string, 1+r.IntN(10))
		for j := range text {
			text[j] = words[r.IntN(len(words))]
		}
		if r.IntN(2000) == 0 {
			text[0] = "ornitorinc"
		}
		chat := chats[r.IntN(len(chats))]
		batch = append(batch, text2(fmt.Sprintf("3EB0%08X", i), chat, chat, int64(1700000000+i*30), strings.Join(text, " ")))
		if len(batch) == cap(batch) {
			if err := s.Apply(b.Context(), store.Changes{Messages: batch}); err != nil {
				b.Fatal(err)
			}
			batch = batch[:0]
		}
	}
	return s
}

func text2(id string, chat, author node.JID, at int64, body string) store.Message {
	return text(id, chat, author, at, body)
}

func BenchmarkStoreAtScale(b *testing.B) {
	s := benchmarkStore(b, 200_000)
	chat := node.JID{User: "40720000007", Server: node.ServerUser}
	for _, bench := range []struct {
		name string
		q    store.Query
	}{
		{"rare word", store.Query{Text: "ornitorinc", Limit: 20}},
		{"common word", store.Query{Text: "meeting", Limit: 20}},
		{"without accents", store.Query{Text: "sanatate maine", Limit: 20}},
		{"one chat", store.Query{Chat: chat, Limit: 50}},
		{"one chat, paged back", store.Query{Chat: chat, Limit: 50, Before: time.Unix(1700000000+100_000*30, 0)}},
	} {
		b.Run(bench.name, func(b *testing.B) {
			for b.Loop() {
				if _, err := s.Messages(b.Context(), bench.q); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("chat list", func(b *testing.B) {
		for b.Loop() {
			if _, err := s.Chats(b.Context(), 30); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMessagesThatRepeatAKnownPair(b *testing.B) {
	s := benchmarkStore(b, 100000)
	pairs := map[node.JID]node.JID{}
	for i := range 300 {
		pairs[node.JID{User: fmt.Sprintf("9%07d", i), Server: node.ServerLID}] = node.JID{User: fmt.Sprintf("4072%07d", i), Server: node.ServerUser}
	}
	if err := s.Apply(b.Context(), store.Changes{LIDs: pairs}); err != nil {
		b.Fatal(err)
	}
	lid, pn := node.JID{User: "90000007", Server: node.ServerLID}, node.JID{User: "40720000007", Server: node.ServerUser}
	b.Run("known pair", func(b *testing.B) {
		i := 0
		for b.Loop() {
			i++
			m := text2(fmt.Sprintf("3EB1%08X", i), lid, lid, int64(1790000000+i), "hello")
			if err := s.Apply(b.Context(), store.Changes{Messages: []store.Message{m}, LIDs: map[node.JID]node.JID{lid: pn}}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("new pair", func(b *testing.B) {
		i := 0
		for b.Loop() {
			i++
			fresh := node.JID{User: fmt.Sprintf("8%07d", i), Server: node.ServerLID}
			m := text2(fmt.Sprintf("3EB2%08X", i), fresh, fresh, int64(1790000000+i), "hello")
			if err := s.Apply(b.Context(), store.Changes{Messages: []store.Message{m}, LIDs: map[node.JID]node.JID{fresh: {User: fmt.Sprintf("4079%07d", i), Server: node.ServerUser}}}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
