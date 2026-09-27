package mcptools

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
)

const maxMentionWords = 3

func withMentions(ctx context.Context, s Sender, dir directory, to node.JID, text string) (string, []node.JID) {
	if to.Server != node.ServerGroup || !strings.Contains(text, "@") {
		return text, nil
	}
	all, err := s.Groups(ctx)
	if err != nil {
		return text, nil
	}
	for _, g := range all {
		if g.JID == to {
			return mentionsIn(dir, g, text)
		}
	}
	return text, nil
}

type mentionable struct {
	address node.JID
	phone   string
	names   []string
}

func members(dir directory, g groups.Group) []mentionable {
	out := make([]mentionable, 0, len(g.Participants))
	for _, p := range g.Participants {
		m := mentionable{address: p.JID, names: namesOf(dir, p)}
		if p.JID.Server == node.ServerUser {
			m.phone = p.JID.User
		}
		if g.AddressingMode == "lid" && p.LID.User != "" || m.address.Server == "" {
			m.address = p.LID
		}
		out = append(out, m)
	}
	return out
}

func namesOf(dir directory, p groups.Participant) []string {
	var names []string
	add := func(name string) {
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, j := range []node.JID{p.JID, p.LID} {
		if j.Server == "" {
			continue
		}
		saved := dir.names[dir.canonical(j)]
		for _, name := range []string{dir.name(j), saved.First, saved.Push} {
			full := fold(name)
			first, _, _ := strings.Cut(full, " ")
			add(full)
			add(first)
		}
	}
	return names
}

func mentionsIn(dir directory, g groups.Group, text string) (string, []node.JID) {
	people := members(dir, g)
	var (
		out       strings.Builder
		mentioned []node.JID
	)
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '@' || i > 0 && !boundary(runes[i-1]) {
			out.WriteRune(runes[i])
			continue
		}
		who, used := mention(people, runes[i+1:])
		if used == 0 {
			out.WriteRune(runes[i])
			continue
		}
		out.WriteString("@" + who.User)
		if !slices.Contains(mentioned, who) {
			mentioned = append(mentioned, who)
		}
		i += used
	}
	return out.String(), mentioned
}

func mention(people []mentionable, after []rune) (node.JID, int) {
	digits := after
	if len(digits) > 0 && digits[0] == '+' {
		digits = digits[1:]
	}
	n := 0
	for n < len(digits) && unicode.IsDigit(digits[n]) {
		n++
	}
	if n > 0 {
		number := string(digits[:n])
		for _, p := range people {
			if p.phone == number {
				return p.address, len(after) - len(digits) + n
			}
		}
		return node.JID{}, 0
	}
	words := wordsAt(after)
	for k := min(len(words), maxMentionWords); k > 0; k-- {
		want := fold(string(after[:words[k-1]]))
		var found node.JID
		matches := 0
		for _, p := range people {
			if slices.Contains(p.names, want) {
				found = p.address
				matches++
				if matches > 1 {
					break
				}
			}
		}
		if matches == 1 {
			return found, words[k-1]
		}
	}
	return node.JID{}, 0
}

func wordsAt(runes []rune) []int {
	var ends []int
	i := 0
	for len(ends) < maxMentionWords {
		start := i
		for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '-' || runes[i] == '\'') {
			i++
		}
		if i == start {
			break
		}
		ends = append(ends, i)
		if i >= len(runes) || runes[i] != ' ' {
			break
		}
		i++
	}
	return ends
}

func boundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) && r != '@'
}
