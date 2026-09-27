package mcptools

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

type ReadInput struct {
	Chat   string `json:"chat,omitempty" jsonschema:"only this chat: a contact or group name, a mobile number with country code, me, or status for contacts' status updates; leave empty for all chats"`
	From   string `json:"from,omitempty" jsonschema:"only messages sent by this person: a contact name, a mobile number with country code, or me"`
	Query  string `json:"query,omitempty" jsonschema:"only messages containing all these words; case and accents do not matter"`
	Limit  int    `json:"limit,omitempty" jsonschema:"how many messages to return, newest last (default 20, at most 200)"`
	Before string `json:"before,omitempty" jsonschema:"to page back: the id of the oldest message already shown, as the earlier result suggests; a time (RFC 3339) also works. In one chat, when this computer has nothing older, the phone is asked for more (a few seconds)"`
	Unread bool   `json:"unread,omitempty" jsonschema:"only the unread messages, from every chat that has some (or only chat), in the order the phone lists the chats; answers what did I miss in one call"`
}

type ReceivedMessage struct {
	ID        string   `json:"id"`
	ReplyTo   string   `json:"reply_to,omitempty"`
	Quote     string   `json:"quote,omitempty"`
	Status    string   `json:"status,omitempty"`
	Chat      string   `json:"chat"`
	From      string   `json:"from"`
	Name      string   `json:"name,omitempty"`
	Time      string   `json:"time"`
	Text      string   `json:"text"`
	Kind      string   `json:"kind"`
	Media     bool     `json:"media,omitempty"`
	Edited    bool     `json:"edited,omitempty"`
	Forwarded string   `json:"forwarded,omitempty"`
	Reactions []string `json:"reactions,omitempty"`
	FromMe    bool     `json:"from_me"`
}

type ReadReport struct {
	State    string            `json:"state"`
	Messages []ReceivedMessage `json:"messages"`
	Detail   string            `json:"detail"`
}

type GroupsReport struct {
	State  string        `json:"state"`
	Groups []ListedGroup `json:"groups"`
	Detail string        `json:"detail"`
}

type GroupsInput struct {
	Group string `json:"group,omitempty" jsonschema:"a group name; also lists that group's members and description"`
}

type ListedGroup struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Participants int           `json:"participants"`
	Description  string        `json:"description,omitempty"`
	Members      []GroupMember `json:"members,omitempty"`
}

type GroupMember struct {
	Name  string `json:"name"`
	Admin bool   `json:"admin,omitempty"`
}

func listGroups(s Sender) mcp.ToolHandlerFor[GroupsInput, GroupsReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GroupsInput) (*mcp.CallToolResult, GroupsReport, error) {
		refuse := func(state, detail string) (*mcp.CallToolResult, GroupsReport, error) {
			report := GroupsReport{State: state, Groups: []ListedGroup{}, Detail: detail}
			return reply(report)
		}
		if _, linked := s.Self(); !linked {
			return refuse(stateNotLinked, notLinked)
		}
		listCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		listed, err := s.Groups(listCtx)
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not list groups: %v", err))
		}
		if want := strings.TrimSpace(in.Group); want != "" {
			listed = matchGroups(listed, want)
			if len(listed) == 0 {
				return refuse("unknown_group", fmt.Sprintf("The user is in no group called %q. Call list_whatsapp_groups without group to see them all.", want))
			}
		}
		report := GroupsReport{State: "ok", Groups: make([]ListedGroup, len(listed)), Detail: fmt.Sprintf("%d group(s).", len(listed))}
		if strings.TrimSpace(in.Group) == "" {
			for i, g := range listed {
				report.Groups[i] = ListedGroup{ID: g.JID.String(), Name: g.Subject, Participants: len(g.Participants)}
			}
			return nil, report, nil
		}
		dir, err := loadDirectory(listCtx, s)
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not look up contacts: %v", err))
		}
		for i, g := range listed {
			report.Groups[i] = describeGroup(dir, g)
		}
		return nil, report, nil
	}
}

func matchGroups(all []groups.Group, name string) []groups.Group {
	want := fold(name)
	var exact, partial []groups.Group
	for _, g := range all {
		switch subject := fold(g.Subject); {
		case subject == want:
			exact = append(exact, g)
		case strings.Contains(subject, want):
			partial = append(partial, g)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return partial
}

func describeGroup(dir directory, g groups.Group) ListedGroup {
	listed := ListedGroup{ID: g.JID.String(), Name: clean(g.Subject), Participants: len(g.Participants), Description: visible(strings.TrimSpace(g.Description))}
	for _, p := range g.Participants {
		who := p.JID
		if who.Server == "" {
			who = p.LID
		}
		listed.Members = append(listed.Members, GroupMember{Name: dir.label(who), Admin: p.Admin})
	}
	return listed
}

func read(s Sender) mcp.ToolHandlerFor[ReadInput, ReadReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ReadInput) (*mcp.CallToolResult, ReadReport, error) {
		refuse := func(state, detail string) (*mcp.CallToolResult, ReadReport, error) {
			report := ReadReport{State: state, Messages: []ReceivedMessage{}, Detail: detail}
			return reply(report)
		}
		if _, linked := s.Self(); !linked {
			return refuse(stateNotLinked, notLinked)
		}
		dir, err := loadDirectory(ctx, s)
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not read the chats: %v", err))
		}
		q, refused := readQuery(ctx, s, &dir, in)
		if refused != nil {
			return refuse(refused.state, refused.detail)
		}
		found, err := fetch(ctx, s, dir, q, in.Unread)
		if errors.Is(err, store.ErrNoSuchMessage) {
			return refuse("invalid_before", fmt.Sprintf("There is no message %q in %s. before takes the id of a message from earlier results, or a time like 2026-09-27T10:00:00Z.", q.BeforeID, cmp.Or(strings.TrimSpace(in.Chat), "any chat")))
		}
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not read messages: %v", err))
		}
		fromPhone := -1
		if pagingOneChat(in, q) && len(found) < q.Limit {
			if fromPhone, err = s.Older(ctx, q.Chat); err == nil && fromPhone > 0 {
				if more, err := s.Messages(ctx, q); err == nil {
					found = more
				}
			}
		}
		messages := make([]ReceivedMessage, 0, len(found))
		for _, m := range found {
			messages = append(messages, describeMessage(dir, m))
		}
		report := ReadReport{State: "ok", Messages: messages, Detail: readDetail(in, q, messages)}
		switch {
		case fromPhone > 0:
			report.Detail += fmt.Sprintf(" %d older message(s) were fetched from the phone.", fromPhone)
		case fromPhone == 0:
			report.Detail += " The phone was asked for older messages but sent none; it may be offline or keep nothing older for this chat."
		}
		return nil, report, nil
	}
}

func pagingOneChat(in ReadInput, q store.Query) bool {
	return !in.Unread && q.Chat.Server != "" && strings.TrimSpace(in.Before) != "" && strings.TrimSpace(in.Query) == "" && len(q.From) == 0 && !q.Chat.IsStatus()
}

func fetch(ctx context.Context, s Sender, dir directory, q store.Query, unreadOnly bool) ([]store.Message, error) {
	if unreadOnly {
		return unread(ctx, s, dir, q.Chat)
	}
	return s.Messages(ctx, q)
}

const kindViewOnce = "view_once"

func readDetail(in ReadInput, q store.Query, messages []ReceivedMessage) string {
	detail := countDetail(in, q, messages)
	if slices.ContainsFunc(messages, func(m ReceivedMessage) bool { return m.Kind == kindViewOnce }) {
		detail += " View-once messages open only on the phone."
	}
	return detail
}

func countDetail(in ReadInput, q store.Query, messages []ReceivedMessage) string {
	switch {
	case in.Unread && len(messages) == 0:
		return "No unread messages."
	case in.Unread:
		return fmt.Sprintf("%d unread message(s).", len(messages))
	case len(messages) == 0 && in.Query != "":
		return "No messages contain those words."
	case len(messages) == 0 && in.Before != "":
		return "No older messages on this computer."
	case len(messages) == 0 && q.Chat.Server != "":
		return "No messages from this chat have reached this computer. Linking brings only the phone's recent history, so an older chat can be empty here."
	case len(messages) == 0:
		return "No messages yet. History from the phone arrives in the first minutes after linking."
	case len(messages) == q.Limit:
		return fmt.Sprintf("%d message(s). There may be older ones: call again with before=%s.", len(messages), messages[0].ID)
	}
	return fmt.Sprintf("%d message(s).", len(messages))
}

func unread(ctx context.Context, s Sender, dir directory, only node.JID) ([]store.Message, error) {
	var out []store.Message
	for _, c := range dir.chats {
		room := maxRead - len(out)
		if room <= 0 {
			break
		}
		if c.Unread <= 0 || only.Server != "" && c.JID != only {
			continue
		}
		found, err := s.Messages(ctx, store.Query{Chat: c.JID, Limit: min(c.Unread, room), Incoming: true})
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

func readQuery(ctx context.Context, s Sender, dir *directory, in ReadInput) (store.Query, *refusal) {
	q := store.Query{Text: in.Query, Limit: min(cmp.Or(in.Limit, defaultRead), maxRead)}
	if in.Limit < 0 {
		return q, &refusal{state: "invalid_limit", detail: fmt.Sprintf("limit must be between 1 and %d.", maxRead)}
	}
	if in.Unread && (in.Query != "" || in.Before != "" || in.From != "") {
		return q, &refusal{state: "invalid_unread", detail: "unread cannot be combined with query, before or from."}
	}
	if from := strings.TrimSpace(in.From); from != "" {
		who, refused := resolve(ctx, s, dir, from, "sender")
		switch {
		case refused != nil:
			return q, refused
		case who.Server == node.ServerGroup || who.Server == node.ServerBroadcast:
			return q, &refusal{state: "invalid_sender", detail: "from must be a person, not a group; use chat for the group."}
		case dir.canonical(who) == dir.self:
			q.Mine = true
		default:
			q.From = []node.JID{who}
		}
	}
	switch strings.ToLower(strings.TrimSpace(in.Chat)) {
	case "":
	case "status", "statuses", "status updates":
		q.Chat = node.StatusBroadcast()
	default:
		chat, refused := resolve(ctx, s, dir, in.Chat, "chat")
		if refused != nil {
			return q, refused
		}
		q.Chat = chat
	}
	if before := strings.TrimSpace(in.Before); before != "" {
		if at, err := time.Parse(time.RFC3339, before); err == nil {
			q.Before = at
		} else {
			q.BeforeID = before
		}
	}
	return q, nil
}

type ChatsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"how many chats to list, most recent first (default 30, at most 500)"`
}

type ListedChat struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Group      bool   `json:"group"`
	Unread     int    `json:"unread,omitempty"`
	LastActive string `json:"last_active,omitempty"`
	Pinned     bool   `json:"pinned,omitempty"`
	Muted      bool   `json:"muted,omitempty"`
	Archived   bool   `json:"archived,omitempty"`
}

type ChatsReport struct {
	State  string       `json:"state"`
	Chats  []ListedChat `json:"chats"`
	Detail string       `json:"detail"`
}

func listChats(s Sender) mcp.ToolHandlerFor[ChatsInput, ChatsReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ChatsInput) (*mcp.CallToolResult, ChatsReport, error) {
		refuse := func(state, detail string) (*mcp.CallToolResult, ChatsReport, error) {
			report := ChatsReport{State: state, Chats: []ListedChat{}, Detail: detail}
			return reply(report)
		}
		if _, linked := s.Self(); !linked {
			return refuse(stateNotLinked, notLinked)
		}
		dir, err := loadDirectory(ctx, s)
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not read the chats: %v", err))
		}
		limit := in.Limit
		if limit <= 0 {
			limit = defaultChats
		}
		limit = min(limit, len(dir.chats))
		report := ChatsReport{State: "ok", Chats: make([]ListedChat, 0, limit), Detail: fmt.Sprintf("%d chat(s).", limit)}
		if limit == 0 {
			report.Detail = "No chats yet. History from the phone arrives in the first minutes after linking."
		}
		for _, c := range dir.chats[:limit] {
			report.Chats = append(report.Chats, listChat(dir, c, time.Now()))
		}
		return nil, report, nil
	}
}

func listChat(dir directory, c store.Chat, now time.Time) ListedChat {
	listed := ListedChat{
		ID: c.JID.String(), Name: dir.label(c.JID), Group: c.JID.Server == node.ServerGroup, Unread: c.Unread,
		Pinned: !c.Pinned.IsZero(), Muted: c.Mute.Active(now), Archived: c.Archived,
	}
	if c.LastMessage.Unix() > 0 {
		listed.LastActive = c.LastMessage.UTC().Format(time.RFC3339)
	}
	return listed
}

func describeMessage(dir directory, m store.Message) ReceivedMessage {
	inner := media.Unwrap(m.Message)
	out := ReceivedMessage{ID: m.ID, Chat: dir.label(m.Chat), Time: m.Time.UTC().Format(time.RFC3339), Kind: message.TypeText, FromMe: m.FromMe}
	body := store.Text(inner)
	ref, hasMedia := media.ReferenceOf(inner)
	sharedKind, shared, isShared := sharedContent(dir, m, inner)
	switch {
	case m.Revoked:
		out.Kind, body = "deleted", ""
	case message.IsViewOnceStub(m.Message):
		out.Kind = kindViewOnce
	case hasMedia:
		out.Kind, out.Media, body = string(ref.Type), true, ref.Caption
	case isShared:
		out.Kind, body = sharedKind, shared
	case body == "":
		out.Kind = message.TypeOf(inner)
	}
	if !m.Revoked {
		context := message.ContextOf(inner)
		body = dir.mentioned(body, context.GetMentionedJid())
		switch {
		case context.GetForwardingScore() >= manyForwards:
			out.Forwarded = "many times"
		case context.GetIsForwarded():
			out.Forwarded = "once"
		}
		out.Edited = !m.Edited.IsZero()
		if q, ok := message.QuoteOf(inner); ok {
			out.ReplyTo, out.Quote = q.ID, fmt.Sprintf("%s: %q", dir.who(q.Author), clip(visible(quoted(q.Message)), maxQuoted))
		}
	}
	out.Text = visible(body)
	for _, r := range m.Reactions {
		out.Reactions = append(out.Reactions, r.Emoji+" "+dir.who(r.By))
	}
	if m.FromMe && m.Status > store.StatusSent {
		out.Status = [...]string{"sent", "delivered", "read", "played"}[min(int(m.Status), 3)]
	}
	pushName := clean(m.PushName)
	out.From, out.Name = dir.label(m.Author), pushName
	if dir.name(m.Author) == "" && pushName != "" {
		out.From = pushName + " (" + display(m.Author) + ")"
	}
	return out
}

func clip(text string, limit int) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return string(runes)
}

func display(j node.JID) string {
	if j.Server == node.ServerUser {
		return "+" + j.User
	}
	return j.WithoutDevice().String()
}
