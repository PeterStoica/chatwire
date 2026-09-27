package mcptools

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/store"
)

const (
	maxWait     = 120 * time.Second
	sendTimeout = 45 * time.Second
	fileTimeout = 5 * time.Minute
)

type Sender interface {
	Self() (node.JID, bool)
	Connection() messenger.Connection
	SendText(ctx context.Context, to node.JID, text string, mentions ...node.JID) (string, error)
	SendPoll(ctx context.Context, to node.JID, question string, options []string, multiple bool) (string, error)
	Reply(ctx context.Context, to node.JID, text, quotedID string, mentions ...node.JID) (string, node.JID, error)
	ChatOf(ctx context.Context, id string) (node.JID, error)
	SendFile(ctx context.Context, to node.JID, f messenger.File) (string, error)
	MarkRead(ctx context.Context, messages []store.Message) error
	Groups(ctx context.Context) ([]groups.Group, error)
	Media(ctx context.Context, id string) (media.Reference, []byte, error)
	Messages(ctx context.Context, q store.Query) ([]store.Message, error)
	Chats(ctx context.Context, limit int) ([]store.Chat, error)
	Names(ctx context.Context) (map[node.JID]store.Name, error)
	LIDs(ctx context.Context) (map[node.JID]node.JID, error)
	React(ctx context.Context, id, emoji string) (store.Message, error)
	Edit(ctx context.Context, id, text string) (store.Message, error)
	Delete(ctx context.Context, id string) (store.Message, error)
	Vote(ctx context.Context, id string, options []string) (store.Message, []string, error)
	Forward(ctx context.Context, to node.JID, id string) (string, store.Message, error)
}

type Options struct {
	MediaDir string
	Folders  []string
	Private  []string
	LinkPage func(context.Context) (url string, opened bool, err error)
}

const Instructions = "Reads and sends WhatsApp messages from the user's own account through a linked device. " +
	"If whatsapp_status says it is not linked, ask the user for their mobile number with country code, call link_whatsapp, show the code exactly as returned, then wait with whatsapp_status. " +
	"To catch up, call read_whatsapp_messages with unread=true. Chats are named the way the user names them: a contact or group name, a number with country code, me, or status. " +
	"Message ids from reads work with reply_to, get_whatsapp_media and change_whatsapp_message. " +
	"Send, react, vote, edit or delete only when the user asked for it, and send exactly the text they approved."

const sendFileDescription = "Send a photo, video, voice note (.opus/.ogg) or any document from this computer over WhatsApp, with an optional caption. to is a contact or group name, a mobile number with country code, or me. " +
	"For safety, files are only sent from the Desktop, Documents, Downloads, Pictures, Movies, Videos, Music and temporary folders (the user can allow more with the CHATWIRE_FILES setting); hidden files never."

func NewServer(impl *mcp.Implementation, l Linker, s Sender, opts Options) *mcp.Server {
	server := mcp.NewServer(impl, &mcp.ServerOptions{Instructions: Instructions})
	Register(server, l, s, opts)
	return server
}

func Register(server *mcp.Server, l Linker, s Sender, opts Options) {
	add(server, &mcp.Tool{
		Name:        "get_whatsapp_media",
		Description: "Open the photo, video, voice note, sticker or document of a WhatsApp message by its id from read_whatsapp_messages. Photos come back as an image; other files are saved and their path is returned.",
	}, getMedia(s, opts))
	add(server, &mcp.Tool{
		Name:        "send_whatsapp_message",
		Description: "Send a WhatsApp message from the user's linked account: text, a reply, a forward, or a poll. to is a contact or group name, a mobile number with country code, or me for the user's own chat.",
	}, send(s))
	add(server, &mcp.Tool{
		Name:        "send_whatsapp_file",
		Description: sendFileDescription,
	}, sendFile(s, newGate(opts)))
	add(server, &mcp.Tool{
		Name: "change_whatsapp_message",
		Description: "React to a WhatsApp message, vote in a poll, or edit or delete one of the user's own messages, by the message id from read_whatsapp_messages. " +
			"Give exactly one of react, remove_reaction, edit, delete or vote. Deleting removes the message for everyone, so do it only when the user asks.",
	}, change(s))
	add(server, &mcp.Tool{
		Name:        "list_whatsapp_chats",
		Description: "List the user's WhatsApp chats as the phone orders them: pinned first, then most recent, archived last; with their names, when each was last active, how many messages are unread, and which are pinned, muted or archived.",
	}, listChats(s))
	add(server, &mcp.Tool{
		Name:        "list_whatsapp_groups",
		Description: "List the WhatsApp groups the user is in, with their names, ids and sizes; with group, also that group's members (admins marked) and description.",
	}, listGroups(s))
	add(server, &mcp.Tool{
		Name: "read_whatsapp_messages",
		Description: "Read WhatsApp messages, newest last, including history synced from the phone. " +
			"Optionally one chat (a contact or group name, a number, me, or status for status updates), only one person's messages (from), only the unread ones, only messages containing some words, and paging back with before.",
	}, read(s))
	add(server, &mcp.Tool{
		Name: "link_whatsapp",
		Description: "Connect the user's WhatsApp to this computer so messages can be read and sent. " +
			"Ask the user for their WhatsApp mobile number with country code and pass it as phone_number: the result is an 8-character code they type on the phone in WhatsApp's Linked devices screen (Link a device, then Link with phone number instead), with no QR code to scan. " +
			"Show the code to the user exactly as returned, then call whatsapp_status with wait_seconds to learn when linking finished. " +
			"Calling again with the same number while linking is in progress returns the same code. Without a phone number this returns a QR code image to scan instead.",
	}, link(l, opts))
	add(server, &mcp.Tool{
		Name:        "whatsapp_status",
		Description: "Tell whether WhatsApp is linked and connected. With wait_seconds it waits for a link in progress to finish, so call it right after showing the user a linking code.",
	}, status(l, s))
}

type refusal struct {
	state  string
	detail string
}

func resolve(ctx context.Context, s Sender, dir *directory, to string) (node.JID, *refusal) {
	to = strings.TrimSpace(to)
	switch {
	case to == "":
		return node.JID{}, &refusal{state: "unknown_recipient", detail: "Say who the message is for: a name, a number with country code, or me."}
	case strings.EqualFold(to, "me") || strings.EqualFold(to, "myself"):
		return dir.self, nil
	}
	if phone, err := pairing.ParsePhone(to); err == nil {
		return node.JID{User: string(phone), Server: node.ServerUser}, nil
	}
	if jid, err := node.ParseJID(to); err == nil {
		return jid, nil
	}
	found := dir.find(to)
	if len(found) == 0 {
		if _, err := s.Groups(ctx); err == nil {
			if refreshed, err := loadDirectory(ctx, s); err == nil {
				*dir, found = refreshed, refreshed.find(to)
			}
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return node.JID{}, &refusal{state: "unknown_recipient", detail: fmt.Sprintf("No contact, group or chat is called %q. Call list_whatsapp_chats to see names, or use a number with country code.", to)}
	default:
		labels := make([]string, 0, min(len(found), maxCandidates))
		for _, j := range found[:min(len(found), maxCandidates)] {
			labels = append(labels, dir.label(j))
		}
		return node.JID{}, &refusal{state: "ambiguous_recipient", detail: fmt.Sprintf("%d chats match %q: %s. Ask the user which one.", len(found), to, strings.Join(labels, "; "))}
	}
}

const (
	defaultRead         = 20
	maxRead             = 200
	defaultChats        = 30
	maxChats            = 500
	maxCandidates       = 10
	maxQuoted           = 60
	manyForwards        = 5
	stateNotLinked      = "not_linked"
	stateFailed         = "failed"
	stateRestricted     = "restricted"
	stateSent           = "sent"
	notLinked           = "WhatsApp is not linked yet. Call link_whatsapp first."
	noSuchMessage       = "No message has that id. Call read_whatsapp_messages for ids."
	stateUnknownMessage = "unknown_message"
	stateDeletedMessage = "deleted_message"
)

func withJSON(report any, content ...mcp.Content) *mcp.CallToolResult {
	raw, err := json.Marshal(report)
	if err != nil {
		return &mcp.CallToolResult{Content: content}
	}
	return &mcp.CallToolResult{Content: append(content, &mcp.TextContent{Text: string(raw)})}
}

func add[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	if schema, err := jsonschema.For[In](nil); err == nil {
		portable(schema)
		tool.InputSchema = schema
	}
	mcp.AddTool(server, tool, handler)
}

func portable(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if kept := slices.DeleteFunc(slices.Clone(s.Types), func(t string) bool { return t == "null" }); len(s.Types) > 0 && len(kept) == 1 {
		s.Type, s.Types = kept[0], nil
	}
	for _, p := range s.Properties {
		portable(p)
	}
	portable(s.Items)
	portable(s.AdditionalProperties)
	for _, d := range s.Defs {
		portable(d)
	}
}
