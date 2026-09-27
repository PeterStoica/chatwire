package mcptools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
)

type SendInput struct {
	To      string     `json:"to,omitempty" jsonschema:"who to message: a contact or group name, a mobile number with country code (for example +40 721 234 567), or me for the user's own chat; may be left empty with reply_to to answer in that message's chat"`
	Text    string     `json:"text,omitempty" jsonschema:"the message text; in a group, @Name or @number mentions that member"`
	ReplyTo string     `json:"reply_to,omitempty" jsonschema:"the id of a message from read_whatsapp_messages to reply to; the reply quotes it"`
	Forward string     `json:"forward,omitempty" jsonschema:"instead of text, the id of a message from read_whatsapp_messages to forward to 'to'; photos, videos, voice notes and files go as they are"`
	Poll    *PollInput `json:"poll,omitempty" jsonschema:"instead of text, a poll for 'to' to vote on"`
}

type PollInput struct {
	Question        string   `json:"question" jsonschema:"the question, at most 255 characters"`
	Options         []string `json:"options" jsonschema:"2 to 12 different answers, each at most 100 characters"`
	MultipleAnswers bool     `json:"multiple_answers,omitempty" jsonschema:"let people pick more than one answer; by default they pick one"`
}

type FileInput struct {
	To      string `json:"to" jsonschema:"who to send it to: a contact or group name, a mobile number with country code, or me"`
	Path    string `json:"path" jsonschema:"the file to send, as a path on this computer; ~/ means the user's home folder"`
	Caption string `json:"caption,omitempty" jsonschema:"text to go with a photo, video or document"`
}

type SendReport struct {
	State  string `json:"state"`
	To     string `json:"to,omitempty"`
	ID     string `json:"id,omitempty"`
	Detail string `json:"detail"`
}

func send(s Sender) mcp.ToolHandlerFor[SendInput, SendReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SendInput) (*mcp.CallToolResult, SendReport, error) {
		if _, linked := s.Self(); !linked {
			return sendReport(SendReport{State: stateNotLinked, Detail: notLinked})
		}
		forward := strings.TrimSpace(in.Forward)
		if in.Poll != nil {
			if forward != "" || strings.TrimSpace(in.Text) != "" || strings.TrimSpace(in.ReplyTo) != "" {
				return sendReport(SendReport{State: "choose_one", Detail: "Give either text (optionally with reply_to), forward or poll."})
			}
			poll := *in.Poll
			return sendReport(deliver(ctx, s, in.To, sendTimeout, "poll", "the poll ", func(ctx context.Context, to node.JID) (string, error) {
				return s.SendPoll(ctx, to, poll.Question, poll.Options, poll.MultipleAnswers)
			}))
		}
		switch {
		case forward != "" && (strings.TrimSpace(in.Text) != "" || strings.TrimSpace(in.ReplyTo) != ""):
			return sendReport(SendReport{State: "choose_one", Detail: "Give either text (optionally with reply_to) or forward, not both."})
		case forward != "":
			return sendReport(deliver(ctx, s, in.To, fileTimeout, "message", "the forwarded message ", func(ctx context.Context, to node.JID) (string, error) {
				id, _, err := s.Forward(ctx, to, forward)
				return id, err
			}))
		case strings.TrimSpace(in.Text) == "":
			return sendReport(SendReport{State: "empty_message", Detail: "There is no text to send."})
		}
		quoted := strings.TrimSpace(in.ReplyTo)
		if quoted != "" && strings.TrimSpace(in.To) == "" {
			return sendReport(replyInPlace(ctx, s, in.Text, quoted))
		}
		return sendReport(deliver(ctx, s, in.To, sendTimeout, "message", "", func(ctx context.Context, to node.JID) (string, error) {
			text, mentions := withMentions(ctx, s, to, in.Text)
			if quoted != "" {
				id, _, err := s.Reply(ctx, to, text, quoted, mentions...)
				return id, err
			}
			return s.SendText(ctx, to, text, mentions...)
		}))
	}
}

func sendFile(s Sender) mcp.ToolHandlerFor[FileInput, SendReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in FileInput) (*mcp.CallToolResult, SendReport, error) {
		if _, linked := s.Self(); !linked {
			return sendReport(SendReport{State: stateNotLinked, Detail: notLinked})
		}
		f, refused := readFile(in.Path)
		if refused != nil {
			return sendReport(SendReport{State: refused.state, Detail: refused.detail})
		}
		f.Caption = strings.TrimSpace(in.Caption)
		what := fmt.Sprintf("%s (%s, %d bytes) ", f.Name, media.KindOf(media.Sniff(f.Name, f.Data)), len(f.Data))
		return sendReport(deliver(ctx, s, in.To, fileTimeout, "file", what, func(ctx context.Context, to node.JID) (string, error) {
			return s.SendFile(ctx, to, f)
		}))
	}
}

func replyInPlace(ctx context.Context, s Sender, text, quoted string) SendReport {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	var mentions []node.JID
	if chat, err := s.ChatOf(ctx, quoted); err == nil {
		text, mentions = withMentions(ctx, s, chat, text)
	}
	id, chat, err := s.Reply(ctx, node.JID{}, text, quoted, mentions...)
	switch {
	case errors.Is(err, messenger.ErrUnknownMessage):
		return SendReport{State: stateUnknownMessage, Detail: noSuchMessage}
	case errors.Is(err, messenger.ErrDeleted):
		return SendReport{State: stateDeletedMessage, Detail: "That message was deleted, so it cannot be replied to."}
	}
	name := display(chat)
	if dir, dirErr := loadDirectory(ctx, s); dirErr == nil {
		name = dir.label(chat)
	}
	if state, detail, ok := stopped(err); ok {
		return SendReport{State: state, To: name, Detail: detail}
	}
	if err != nil {
		return SendReport{State: stateFailed, To: name, Detail: fmt.Sprintf("The reply was not sent: %v", err)}
	}
	return SendReport{State: stateSent, To: name, ID: id, Detail: fmt.Sprintf("Replied in %s.", name)}
}

func deliver(ctx context.Context, s Sender, to string, timeout time.Duration, noun, what string, do func(context.Context, node.JID) (string, error)) SendReport {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dir, err := loadDirectory(ctx, s)
	if err != nil {
		return SendReport{State: stateFailed, Detail: fmt.Sprintf("Could not look up contacts: %v", err)}
	}
	target, refusal := resolve(ctx, s, &dir, to)
	if refusal != nil {
		return SendReport{State: refusal.state, Detail: refusal.detail}
	}
	if target.Server == node.ServerBroadcast {
		return SendReport{State: "unsupported_recipient", Detail: "Posting status updates and broadcast lists is not supported; send to a contact or group."}
	}
	name := dir.label(target)
	id, err := do(ctx, target)
	if state, detail, ok := stopped(err); ok {
		return SendReport{State: state, To: name, Detail: detail}
	}
	if state, detail, ok := refusedSend(err); ok {
		return SendReport{State: state, To: name, Detail: detail}
	}
	if err != nil {
		return SendReport{State: stateFailed, To: name, Detail: fmt.Sprintf("The %s was not sent: %v", noun, err)}
	}
	return SendReport{State: stateSent, To: name, ID: id, Detail: fmt.Sprintf("Sent %sto %s.", what, name)}
}

func readFile(path string) (messenger.File, *refusal) {
	path = strings.TrimSpace(path)
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}
	if path == "" {
		return messenger.File{}, &refusal{state: "file_not_found", detail: "Say which file to send, as a path on this computer."}
	}
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return messenger.File{}, &refusal{state: "file_not_found", detail: fmt.Sprintf("There is no file at %s.", path)}
	case info.IsDir():
		return messenger.File{}, &refusal{state: "not_a_file", detail: fmt.Sprintf("%s is a folder; send the files inside it one by one, or zip it first.", path)}
	case info.Size() > client.MaxUpload:
		return messenger.File{}, &refusal{state: "too_large", detail: fmt.Sprintf("%s is %d MB; at most %d MB can be sent.", path, info.Size()>>20, client.MaxUpload>>20)}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return messenger.File{}, &refusal{state: stateFailed, detail: fmt.Sprintf("Could not read %s: %v", path, err)}
	}
	return messenger.File{Name: filepath.Base(path), Data: data}, nil
}

func sendReport(report SendReport) (*mcp.CallToolResult, SendReport, error) {
	return nil, report, nil
}

func stopped(err error) (state, detail string, ok bool) {
	var rejected client.Rejection
	switch {
	case errors.Is(err, messenger.ErrTooFast):
		return "slow_down", fmt.Sprintf("Nothing was sent: at most %d messages go out a minute, so WhatsApp does not take the account for a spammer. Wait a minute, then send the rest.", messenger.MaxPerMinute), true
	case errors.Is(err, messenger.ErrNewChats):
		return "new_chat_limit", fmt.Sprintf("Nothing was sent: this would start a new chat, and %d were started in the last 24 hours. WhatsApp bans accounts that message many new people; send it later, or from the phone.", messenger.MaxNewChats), true
	case errors.Is(err, messenger.ErrRestricted):
		return stateRestricted, "Nothing was sent: WhatsApp refused a message to someone new in the last day (error 463), so messages to new contacts are held for 24 hours. Existing chats still work; ask the user to send this one from the phone.", true
	case errors.As(err, &rejected):
		state, detail := rejection(rejected.Code)
		return state, detail, true
	}
	return "", "", false
}

func rejection(code int) (state, detail string) {
	switch code {
	case client.CodeRestricted:
		return stateRestricted, "WhatsApp refused it (error 463): it limits this account's messages to people who have not chatted with it, often after many messages to new contacts. Do not retry; ask the user to send it from the phone. Messages to new contacts are now held for 24 hours."
	case client.CodeForbidden:
		return "not_allowed", "WhatsApp refused it (error 403): the person has blocked this account, or only admins can send in this group."
	case client.CodeUnsupported:
		return "unsupported", "WhatsApp refused it (error 405): this kind of message cannot be sent from a linked device."
	case client.CodeChatCap:
		return "new_chat_limit", "WhatsApp refused it (error 475): this business account reached its limit of new chats. Try again later."
	case client.CodeMalformed, client.CodeStaleGroup, client.CodeInvalid:
		return "try_again", fmt.Sprintf("WhatsApp refused it (error %d), usually because the chat's devices or group settings just changed. Try once more; if it fails again, ask the user to send it from the phone.", code)
	}
	return "rejected", fmt.Sprintf("WhatsApp refused it (error %d). Do not retry right away; ask the user to check the chat on the phone.", code)
}

func refusedSend(err error) (state, detail string, ok bool) {
	switch {
	case errors.Is(err, messenger.ErrUnknownMessage):
		return stateUnknownMessage, noSuchMessage, true
	case errors.Is(err, messenger.ErrDeleted):
		return stateDeletedMessage, "That message was deleted, so it cannot be replied to or forwarded.", true
	case errors.Is(err, messenger.ErrNoForward):
		return "not_forwardable", "Polls and view-once photos or videos cannot be forwarded.", true
	case errors.Is(err, message.ErrPoll):
		return "invalid_poll", "Not sent: " + strings.TrimPrefix(err.Error(), message.ErrPoll.Error()+": ") + ".", true
	}
	return "", "", false
}
