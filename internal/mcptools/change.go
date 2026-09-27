package mcptools

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

type ChangeInput struct {
	MessageID      string   `json:"message_id" jsonschema:"the id of the message, from read_whatsapp_messages"`
	React          *string  `json:"react,omitempty" jsonschema:"one emoji to react with; it replaces the user's earlier reaction"`
	RemoveReaction bool     `json:"remove_reaction,omitempty" jsonschema:"take the user's reaction back"`
	Edit           *string  `json:"edit,omitempty" jsonschema:"new text for one of the user's own messages, or the new caption of their photo, video or document, at most 15 minutes after it was sent"`
	Delete         bool     `json:"delete,omitempty" jsonschema:"delete one of the user's own messages for everyone, at most 2.5 days after it was sent; only when the user asks"`
	Vote           []string `json:"vote,omitempty" jsonschema:"the poll options to vote for, by name; replaces the user's earlier vote"`
	RemoveVote     bool     `json:"remove_vote,omitempty" jsonschema:"take the user's vote on a poll back"`
}

type ChangeReport struct {
	State  string `json:"state"`
	Chat   string `json:"chat,omitempty"`
	Detail string `json:"detail"`
}

func change(s Sender) mcp.ToolHandlerFor[ChangeInput, ChangeReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ChangeInput) (*mcp.CallToolResult, ChangeReport, error) {
		if _, linked := s.Self(); !linked {
			return changeReport(ChangeReport{State: stateNotLinked, Detail: notLinked})
		}
		id := strings.TrimSpace(in.MessageID)
		chosen := 0
		for _, set := range []bool{in.React != nil, in.RemoveReaction, in.Edit != nil, in.Delete, len(in.Vote) > 0, in.RemoveVote} {
			if set {
				chosen++
			}
		}
		if id == "" || chosen != 1 {
			return changeReport(ChangeReport{State: "choose_one", Detail: "Give message_id and exactly one of react, remove_reaction, edit, delete, vote or remove_vote."})
		}
		ctx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		var (
			target store.Message
			done   string
			err    error
		)
		switch {
		case in.React != nil:
			react := strings.TrimSpace(*in.React)
			if react == "" {
				return changeReport(ChangeReport{State: "not_an_emoji", Detail: notEmoji})
			}
			target, err = s.React(ctx, id, react)
			done = "Reacted " + react + " to"
		case in.RemoveReaction:
			target, err = s.React(ctx, id, "")
			done = "Took back the reaction to"
		case in.Edit != nil:
			target, err = s.Edit(ctx, id, *in.Edit)
			done = "Edited"
		case in.Delete:
			target, err = s.Delete(ctx, id)
			done = "Deleted for everyone"
		case in.RemoveVote:
			target, _, err = s.Vote(ctx, id, nil)
			done = "Took back the vote in"
		default:
			var votes []string
			target, votes, err = s.Vote(ctx, id, in.Vote)
			done = "Voted " + strings.Join(votes, ", ") + " in"
		}
		return changeReport(changed(ctx, s, target, done, err))
	}
}

func changed(ctx context.Context, s Sender, target store.Message, done string, err error) ChangeReport {
	if state, detail := refusedChange(target, err); state != "" {
		return ChangeReport{State: state, Detail: detail}
	}
	if state, detail, ok := paced(err); ok {
		return ChangeReport{State: state, Detail: detail}
	}
	chat := display(target.Chat)
	if dir, dirErr := loadDirectory(ctx, s); dirErr == nil {
		chat = dir.label(target.Chat)
	}
	if err != nil {
		return ChangeReport{State: stateFailed, Chat: chat, Detail: fmt.Sprintf("Nothing was changed: %v", err)}
	}
	return ChangeReport{State: "done", Chat: chat, Detail: fmt.Sprintf("%s %s in %s.", done, what(target.Message), chat)}
}

func what(m *wire.Message) string {
	if text := quoted(m); text != "" {
		return strconv.Quote(clip(text, maxQuoted))
	}
	inner := media.Unwrap(m)
	if ref, ok := media.ReferenceOf(inner); ok {
		return "the " + mediaNames[ref.Type]
	}
	return "the " + message.TypeOf(inner) + " message"
}

func quoted(m *wire.Message) string {
	if poll := message.PollOf(media.Unwrap(m)); poll != nil {
		return poll.GetName()
	}
	return store.Text(m)
}

func refusedChange(target store.Message, err error) (string, string) {
	switch {
	case errors.Is(err, messenger.ErrUnknownMessage):
		return stateUnknownMessage, noSuchMessage
	case errors.Is(err, messenger.ErrDeleted):
		return stateDeletedMessage, "That message was deleted, so it cannot be changed."
	case errors.Is(err, messenger.ErrNotMine):
		return "not_yours", "Only the user's own messages can be edited or deleted."
	case errors.Is(err, messenger.ErrTooLate):
		return "too_late", fmt.Sprintf("WhatsApp allows editing for 15 minutes and deleting for everyone for 2.5 days after sending (%v).", err)
	case errors.Is(err, messenger.ErrNotText):
		return "not_text", "Only text, and the captions of photos, videos and documents, can be edited."
	case errors.Is(err, messenger.ErrNotEmoji):
		return "not_an_emoji", notEmoji
	case errors.Is(err, messenger.ErrEmptyEdit):
		return "empty_message", "The new text is empty; to remove the message, delete it instead."
	case errors.Is(err, messenger.ErrNotPoll):
		return "not_a_poll", "That message is not a poll."
	case errors.Is(err, messenger.ErrNoOption), errors.Is(err, messenger.ErrTooMany):
		poll := message.PollOf(media.Unwrap(target.Message))
		options := make([]string, 0, len(poll.GetOptions()))
		for _, o := range poll.GetOptions() {
			options = append(options, o.GetOptionName())
		}
		problem := fmt.Sprintf("This poll allows at most %d choice(s).", poll.GetSelectableOptionsCount())
		if errors.Is(err, messenger.ErrNoOption) {
			problem = strings.TrimPrefix(err.Error(), messenger.ErrNoOption.Error()+": ") + " is not one of its options."
		}
		return "invalid_vote", fmt.Sprintf("%s The options are: %s.", problem, strings.Join(options, " / "))
	case errors.Is(err, messenger.ErrNoSecret):
		return "cannot_vote", "This poll arrived without the key its votes need, so votes cannot be sent from here."
	}
	return "", ""
}

const notEmoji = "A reaction must be a single emoji, like 👍 or ❤️."

var mediaNames = map[media.Type]string{
	media.Image: "photo", media.Sticker: "sticker", media.Video: "video", media.GIF: "GIF", media.Audio: "audio",
	media.Voice: "voice note", media.Document: "document",
}

func changeReport(report ChangeReport) (*mcp.CallToolResult, ChangeReport, error) {
	return nil, report, nil
}
