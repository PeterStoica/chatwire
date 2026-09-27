package store

import (
	"strings"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func match(text string) string {
	words := strings.Fields(text)
	for i, w := range words {
		words[i] = `"` + strings.ReplaceAll(w, `"`, `""`) + `"*`
	}
	return strings.Join(words, " ")
}

func Text(m *wire.Message) string {
	var parts []string
	add := func(fields ...string) {
		for _, field := range fields {
			if field = strings.TrimSpace(field); field != "" {
				parts = append(parts, field)
			}
		}
	}
	m = media.Unwrap(m)
	poll := message.PollOf(m)
	add(m.GetConversation(), m.GetExtendedTextMessage().GetText(), m.GetImageMessage().GetCaption(), m.GetVideoMessage().GetCaption(),
		m.GetDocumentMessage().GetFileName(), m.GetDocumentMessage().GetCaption(), m.GetContactMessage().GetDisplayName(),
		m.GetContactsArrayMessage().GetDisplayName(), m.GetLocationMessage().GetName(), m.GetLocationMessage().GetAddress(),
		m.GetLocationMessage().GetComment(), m.GetLiveLocationMessage().GetCaption(), poll.GetName())
	for _, contact := range m.GetContactsArrayMessage().GetContacts() {
		add(contact.GetDisplayName())
	}
	for _, option := range poll.GetOptions() {
		add(option.GetOptionName())
	}
	return strings.Join(parts, "\n")
}
