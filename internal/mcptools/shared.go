package mcptools

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func sharedContent(dir directory, stored store.Message, m *wire.Message) (kind, body string, ok bool) {
	switch poll := message.PollOf(m); {
	case m.GetLocationMessage() != nil:
		l := m.GetLocationMessage()
		kind = "location"
		if l.GetIsLive() {
			kind = "live_location"
		}
		return kind, place(kind, l.GetDegreesLatitude(), l.GetDegreesLongitude(), l.GetName(), l.GetAddress(), l.GetComment(), l.GetUrl()), true
	case m.GetLiveLocationMessage() != nil:
		l := m.GetLiveLocationMessage()
		return "live_location", place("live_location", l.GetDegreesLatitude(), l.GetDegreesLongitude(), l.GetCaption()), true
	case m.GetContactMessage() != nil:
		return "contact", "[contact] " + card(m.GetContactMessage()), true
	case m.GetContactsArrayMessage() != nil:
		cards := make([]string, 0, len(m.GetContactsArrayMessage().GetContacts()))
		for _, c := range m.GetContactsArrayMessage().GetContacts() {
			cards = append(cards, card(c))
		}
		return "contact", "[contacts] " + strings.Join(cards, "; "), true
	case poll != nil:
		return "poll", "[poll] " + poll.GetName() + " Options: " + tally(dir, poll, stored.Votes), true
	}
	return "", "", false
}

func tally(dir directory, poll *wire.Message_PollCreationMessage, votes []store.Vote) string {
	voters := make(map[string][]string, len(poll.GetOptions()))
	for _, v := range votes {
		who := dir.who(v.By)
		for _, option := range v.Options {
			voters[option] = append(voters[option], who)
		}
	}
	options := make([]string, 0, len(poll.GetOptions()))
	for _, o := range poll.GetOptions() {
		name, by := o.GetOptionName(), voters[o.GetOptionName()]
		switch {
		case len(votes) == 0:
			options = append(options, name)
		case len(by) == 0:
			options = append(options, name+" (0)")
		default:
			options = append(options, name+" ("+strconv.Itoa(len(by))+": "+strings.Join(by, ", ")+")")
		}
	}
	return strings.Join(options, " / ")
}

func place(kind string, latitude, longitude float64, details ...string) string {
	var named []string
	for _, d := range details {
		if d = strings.TrimSpace(d); d != "" {
			named = append(named, d)
		}
	}
	head := "[" + strings.ReplaceAll(kind, "_", " ") + " " + strconv.FormatFloat(latitude, 'f', -1, 64) + ", " + strconv.FormatFloat(longitude, 'f', -1, 64) + "]"
	return strings.TrimSpace(head + " " + strings.Join(named, ", "))
}

func card(c *wire.Message_ContactMessage) string {
	name := cmp.Or(strings.TrimSpace(c.GetDisplayName()), "unnamed")
	if numbers := phones(c.GetVcard()); len(numbers) > 0 {
		return name + ": " + strings.Join(numbers, ", ")
	}
	return name
}

func phones(vcard string) []string {
	var out []string
	for line := range strings.Lines(vcard) {
		property, value, found := strings.Cut(strings.TrimSpace(line), ":")
		property, _, _ = strings.Cut(property, ";")
		if _, grouped, ok := strings.Cut(property, "."); ok {
			property = grouped
		}
		if value = strings.TrimSpace(strings.TrimPrefix(value, "tel:")); found && strings.EqualFold(property, "TEL") && value != "" {
			out = append(out, value)
		}
	}
	return out
}
