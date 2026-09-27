package mcptools

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

type directory struct {
	self  node.JID
	chats []store.Chat
	named map[node.JID]string
	names map[node.JID]store.Name
	lids  map[node.JID]node.JID
}

func loadDirectory(ctx context.Context, s Sender) (directory, error) {
	self, _ := s.Self()
	chats, err := s.Chats(ctx, maxChats)
	if err != nil {
		return directory{}, err
	}
	names, err := s.Names(ctx)
	if err != nil {
		return directory{}, err
	}
	lids, err := s.LIDs(ctx)
	if err != nil {
		return directory{}, err
	}
	for i := range chats {
		chats[i].Name = clean(chats[i].Name)
	}
	for j, n := range names {
		names[j] = store.Name{Contact: clean(n.Contact), First: clean(n.First), Push: clean(n.Push)}
	}
	d := directory{self: self, chats: chats, named: make(map[node.JID]string, len(chats)), names: names, lids: lids}
	for _, c := range chats {
		if c.Name != "" {
			d.named[c.JID] = c.Name
		}
	}
	return d, nil
}

func bare(j node.JID) node.JID {
	return j.WithoutDevice()
}

func (d directory) canonical(j node.JID) node.JID {
	j = bare(j)
	if pn, ok := d.lids[j]; ok {
		return pn
	}
	return j
}

func (d directory) name(j node.JID) string {
	for _, candidate := range []node.JID{bare(j), d.canonical(j)} {
		n := d.names[candidate]
		if found := cmp.Or(d.named[candidate], n.Contact, n.First, n.Push); found != "" {
			return found
		}
	}
	return ""
}

func (d directory) forms(j node.JID) []node.JID {
	j = bare(j)
	out := []node.JID{j}
	for lid, pn := range d.lids {
		switch j {
		case lid:
			out = append(out, pn)
		case pn:
			out = append(out, lid)
		}
	}
	return out
}

func (d directory) who(j node.JID) string {
	if d.canonical(j) == d.self {
		return "me"
	}
	return cmp.Or(d.name(j), display(d.canonical(j)))
}

func (d directory) mentioned(text string, jids []string) string {
	users := make(map[string]node.JID, len(jids))
	for _, raw := range jids {
		if j, err := node.ParseJID(raw); err == nil && j.User != "" {
			users[j.User] = j
		}
	}
	if len(users) == 0 {
		return text
	}
	var out strings.Builder
	for {
		at := strings.IndexByte(text, '@')
		if at < 0 {
			out.WriteString(text)
			return out.String()
		}
		end := at + 1
		for end < len(text) && text[end] >= '0' && text[end] <= '9' {
			end++
		}
		out.WriteString(text[:at])
		if j, ok := users[text[at+1:end]]; ok {
			out.WriteString("@" + d.who(j))
		} else {
			out.WriteString(text[at:end])
		}
		text = text[end:]
	}
}

func (d directory) label(j node.JID) string {
	if j.IsStatus() {
		return "status updates"
	}
	shown := display(d.canonical(j))
	if d.canonical(j) == d.self {
		return "me (" + shown + ")"
	}
	if n := d.name(j); n != "" && j.Server != node.ServerGroup {
		return n + " (" + shown + ")"
	} else if n != "" {
		return n
	}
	return shown
}

func (d directory) find(query string) []node.JID {
	want := fold(query)
	if want == "" {
		return nil
	}
	wantPlain := plain(want)
	var exact, undecorated, partial []node.JID
	consider := func(j node.JID, candidates ...string) {
		j = d.canonical(j)
		if slices.Contains(exact, j) {
			return
		}
		for _, c := range candidates {
			switch folded := fold(c); {
			case folded == "":
			case folded == want:
				exact = append(exact, j)
				return
			case wantPlain != "" && plain(folded) == wantPlain && !slices.Contains(undecorated, j):
				undecorated = append(undecorated, j)
			case strings.Contains(folded, want) && !slices.Contains(partial, j):
				partial = append(partial, j)
			}
		}
	}
	for _, c := range d.chats {
		consider(c.JID, c.Name)
	}
	for _, j := range sortedKeys(d.names) {
		n := d.names[j]
		consider(j, n.Contact, n.First, n.Push)
	}
	switch {
	case len(exact) > 0:
		return exact
	case len(undecorated) > 0:
		return undecorated
	}
	return partial
}

func plain(folded string) string {
	return strings.Join(strings.FieldsFunc(folded, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}

func clean(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == 0x200E || r == 0x200F || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s))
}

func sortedKeys(names map[node.JID]store.Name) []node.JID {
	keys := make([]node.JID, 0, len(names))
	for j := range names {
		keys = append(keys, j)
	}
	slices.SortFunc(keys, func(a, b node.JID) int { return cmp.Compare(a.String(), b.String()) })
	return keys
}

func fold(s string) string {
	folded, _, _ := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	return strings.ToLower(strings.Join(strings.Fields(folded), " "))
}
