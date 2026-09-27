package mcptools

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
	f := newFolder()
	want := f.fold(query)
	if want == "" {
		return nil
	}
	wantPlain := plain(want)
	var exact, undecorated, partial []node.JID
	seen := map[node.JID]int{}
	const (
		inPartial = iota + 1
		inUndecorated
		inExact
	)
	consider := func(j node.JID, candidates ...string) {
		j = d.canonical(j)
		if seen[j] == inExact {
			return
		}
		for _, c := range candidates {
			switch folded := f.fold(c); {
			case folded == "":
			case folded == want:
				exact, seen[j] = append(exact, j), inExact
				return
			case seen[j] >= inUndecorated:
			case wantPlain != "" && plain(folded) == wantPlain:
				undecorated, seen[j] = append(undecorated, j), inUndecorated
			case seen[j] == 0 && strings.Contains(folded, want):
				partial, seen[j] = append(partial, j), inPartial
			}
		}
	}
	for _, c := range d.chats {
		consider(c.JID, c.Name)
	}
	for _, j := range slices.SortedFunc(maps.Keys(d.names), byAddress) {
		n := d.names[j]
		consider(j, n.Contact, n.First, n.Push)
	}
	switch {
	case len(exact) > 0:
		return exact
	case len(undecorated) > 0:
		return slices.DeleteFunc(undecorated, func(j node.JID) bool { return seen[j] == inExact })
	}
	return slices.DeleteFunc(partial, func(j node.JID) bool { return seen[j] != inPartial })
}

func byAddress(a, b node.JID) int {
	return cmp.Or(cmp.Compare(a.User, b.User), cmp.Compare(a.Server, b.Server), cmp.Compare(a.Device, b.Device))
}

func plain(folded string) string {
	var b strings.Builder
	b.Grow(len(folded))
	gap := false
	for _, r := range folded {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			gap = b.Len() > 0
			continue
		}
		if gap {
			b.WriteByte(' ')
			gap = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func clean(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == 0x200E || r == 0x200F || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 || isTag(r) {
			return -1
		}
		return r
	}, s))
}

const (
	blackFlag   = 0x1F3F4
	cancelTag   = 0xE007F
	maxFlagTags = 7
)

func isTag(r rune) bool {
	return r >= 0xE0000 && r <= cancelTag
}

func visible(s string) string {
	if !strings.ContainsFunc(s, isTag) {
		return s
	}
	var (
		b         strings.Builder
		run       []rune
		afterFlag bool
	)
	flush := func() {
		last := len(run) - 1
		if afterFlag && last > 0 && last <= maxFlagTags && run[last] == cancelTag && !slices.Contains(run[:last], cancelTag) {
			for _, r := range run {
				b.WriteRune(r)
			}
		}
		run = run[:0]
	}
	for _, r := range s {
		if isTag(r) {
			run = append(run, r)
			continue
		}
		flush()
		afterFlag = r == blackFlag
		b.WriteRune(r)
	}
	flush()
	return b.String()
}

type folder struct {
	decomposed []byte
}

func newFolder() *folder {
	return &folder{}
}

func (f *folder) fold(s string) string {
	text := s
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			f.decomposed = norm.NFD.AppendString(f.decomposed[:0], s)
			text = string(f.decomposed)
			break
		}
	}
	var b strings.Builder
	b.Grow(len(text))
	gap := false
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			gap = b.Len() > 0
			continue
		case r >= utf8.RuneSelf && unicode.Is(unicode.Mn, r):
			continue
		}
		if gap {
			b.WriteByte(' ')
			gap = false
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func fold(s string) string {
	return newFolder().fold(s)
}
