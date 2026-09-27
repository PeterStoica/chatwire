package messenger

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	editWindow   = 15 * time.Minute
	deleteWindow = 60 * time.Hour
)

var (
	ErrNotMine   = errors.New("messenger: only the user's own messages can be changed")
	ErrTooLate   = errors.New("messenger: too late to change that message")
	ErrNotText   = errors.New("messenger: only text and the captions of photos, videos and documents can be edited")
	ErrNotPoll   = errors.New("messenger: that message is not a poll")
	ErrNoOption  = errors.New("messenger: the poll has no such option")
	ErrTooMany   = errors.New("messenger: the poll allows fewer choices")
	ErrNoSecret  = errors.New("messenger: that poll cannot take votes from here")
	ErrEmptyEdit = errors.New("messenger: the new text is empty")
	ErrNotEmoji  = errors.New("messenger: a reaction must be a single emoji")
	ErrNoForward = errors.New("messenger: that message cannot be forwarded")
)

func (m *Messenger) target(ctx context.Context, id string) (store.Message, *wire.MessageKey, error) {
	t, ok, err := m.store.Message(ctx, id)
	switch {
	case err != nil:
		return t, nil, err
	case !ok:
		return t, nil, fmt.Errorf("%w: %s", ErrUnknownMessage, id)
	case t.Revoked:
		return t, nil, fmt.Errorf("%w: %s", ErrDeleted, id)
	}
	key := &wire.MessageKey{RemoteJid: new(t.Chat.String()), FromMe: new(t.FromMe), Id: new(t.ID)}
	if !t.FromMe && (t.Chat.Server == node.ServerGroup || t.Chat.Server == node.ServerBroadcast) {
		key.Participant = new(t.Author.WithoutDevice().String())
	}
	return t, key, nil
}

func (m *Messenger) own(ctx context.Context, id string, window time.Duration) (store.Message, *wire.MessageKey, error) {
	t, key, err := m.target(ctx, id)
	switch {
	case err != nil:
		return t, nil, err
	case !t.FromMe:
		return t, nil, fmt.Errorf("%w: %s", ErrNotMine, id)
	case m.link.Now().Sub(t.Time) > window:
		return t, nil, fmt.Errorf("%w: sent %s ago", ErrTooLate, m.link.Now().Sub(t.Time).Round(time.Minute))
	}
	return t, key, nil
}

func (m *Messenger) React(ctx context.Context, id, emoji string) (store.Message, error) {
	if emoji != "" && !message.OneEmoji(emoji) {
		return store.Message{}, fmt.Errorf("%w: %q", ErrNotEmoji, emoji)
	}
	t, key, err := m.target(ctx, id)
	if err != nil {
		return t, err
	}
	reaction := &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Key: key, Text: new(emoji), SenderTimestampMs: new(m.link.Now().UnixMilli())}}
	_, err = m.sendMessage(ctx, t.Chat, reaction)
	return t, err
}

func (m *Messenger) Edit(ctx context.Context, id, text string) (store.Message, error) {
	if strings.TrimSpace(text) == "" {
		return store.Message{}, ErrEmptyEdit
	}
	t, key, err := m.own(ctx, id, editWindow)
	if err != nil {
		return t, err
	}
	edited := &wire.Message{Conversation: new(text)}
	switch inner := media.Unwrap(t.Message); {
	case inner.GetExtendedTextMessage() != nil:
		edited = &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new(text)}}
	case inner.GetImageMessage() != nil:
		edited = &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new(text)}}
	case inner.GetVideoMessage() != nil:
		edited = &wire.Message{VideoMessage: &wire.Message_VideoMessage{Caption: new(text)}}
	case inner.GetDocumentMessage() != nil:
		edited = captioned(&wire.Message{DocumentMessage: &wire.Message_DocumentMessage{Caption: new(text)}})
	case inner.Conversation == nil:
		return t, fmt.Errorf("%w: %s", ErrNotText, id)
	}
	edit := &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
		Key: key, Type: wire.Message_ProtocolMessage_MESSAGE_EDIT.Enum(), EditedMessage: edited, TimestampMs: new(m.link.Now().UnixMilli()),
	}}
	_, err = m.sendMessage(ctx, t.Chat, edit)
	return t, err
}

func (m *Messenger) Delete(ctx context.Context, id string) (store.Message, error) {
	t, key, err := m.own(ctx, id, deleteWindow)
	if err != nil {
		return t, err
	}
	revoke := &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key, Type: wire.Message_ProtocolMessage_REVOKE.Enum()}}
	_, err = m.sendMessage(ctx, t.Chat, revoke)
	return t, err
}

func (m *Messenger) Vote(ctx context.Context, id string, options []string) (store.Message, []string, error) {
	t, key, err := m.target(ctx, id)
	if err != nil {
		return t, nil, err
	}
	poll := message.PollOf(media.Unwrap(t.Message))
	if poll == nil {
		return t, nil, fmt.Errorf("%w: %s", ErrNotPoll, id)
	}
	chosen, err := choose(poll, options)
	if err != nil {
		return t, nil, err
	}
	creator, voter, err := m.ballotAddresses(ctx, t.Author)
	if err != nil {
		return t, nil, err
	}
	ballot := message.Ballot{Secret: t.Message.GetMessageContextInfo().GetMessageSecret(), PollID: t.ID, Creator: creator, Voter: voter}
	enc, err := message.SealVote(rand.Reader, ballot, chosen)
	if errors.Is(err, message.ErrSecret) {
		return t, nil, fmt.Errorf("%w: %s", ErrNoSecret, id)
	} else if err != nil {
		return t, nil, err
	}
	vote := &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{PollCreationMessageKey: key, Vote: enc, SenderTimestampMs: new(m.link.Now().UnixMilli())}}
	_, err = m.sendMessage(ctx, t.Chat, vote)
	return t, chosen, err
}

func choose(poll *wire.Message_PollCreationMessage, wanted []string) ([]string, error) {
	var chosen []string
	for _, w := range wanted {
		found := ""
		for _, o := range poll.GetOptions() {
			if strings.EqualFold(strings.TrimSpace(w), strings.TrimSpace(o.GetOptionName())) {
				found = o.GetOptionName()
				break
			}
		}
		if found == "" {
			return nil, NoOption{Choice: w}
		}
		if !slices.Contains(chosen, found) {
			chosen = append(chosen, found)
		}
	}
	if limit := int(poll.GetSelectableOptionsCount()); limit != 0 && len(chosen) > limit {
		return nil, fmt.Errorf("%w: at most %d", ErrTooMany, limit)
	}
	return chosen, nil
}

func (m *Messenger) ballotAddresses(ctx context.Context, creator node.JID) (node.JID, node.JID, error) {
	creators, err := m.addresses(ctx, creator)
	if err != nil {
		return node.JID{}, node.JID{}, err
	}
	self, _ := m.Self()
	voters, err := m.addresses(ctx, self)
	if err != nil {
		return node.JID{}, node.JID{}, err
	}
	for _, server := range []node.Server{node.ServerLID, node.ServerUser} {
		c, cOK := on(creators, server)
		v, vOK := on(voters, server)
		if cOK && vOK {
			return c, v, nil
		}
	}
	return creator.WithoutDevice(), self, nil
}

func on(addresses []node.JID, server node.Server) (node.JID, bool) {
	for _, a := range addresses {
		if a.Server == server && a.User != "" {
			return a, true
		}
	}
	return node.JID{}, false
}

func (m *Messenger) Forward(ctx context.Context, to node.JID, id string) (string, store.Message, error) {
	t, _, err := m.target(ctx, id)
	if err != nil {
		return "", t, err
	}
	inner := media.Unwrap(t.Message)
	if message.PollOf(inner) != nil || viewOnce(t.Message) {
		return "", t, fmt.Errorf("%w: %s", ErrNoForward, id)
	}
	score := message.ContextOf(inner).GetForwardingScore() + 1
	ref, hasMedia := media.ReferenceOf(inner)
	var data []byte
	if hasMedia {
		if ref, data, err = m.Media(ctx, id); err != nil {
			return "", t, err
		}
	}
	sent, err := m.send(ctx, to, func(c *client.Client) (*wire.Message, error) {
		content := t.Message
		if hasMedia {
			up, err := c.Upload(ctx, ref.Type, data)
			if err != nil {
				return nil, err
			}
			content, _ = media.Rehost(content, media.Hosted{
				URL: up.URL, DirectPath: up.DirectPath, MediaKey: up.MediaKey, FileSHA256: up.FileSHA256,
				FileEncSHA256: up.FileEncSHA256, FileLength: up.FileLength, Stamp: m.link.Now().Unix(),
			})
		}
		forwarded, ok := message.Forwarded(content, score)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNoForward, id)
		}
		if forwarded.GetDocumentMessage().GetCaption() != "" {
			forwarded = captioned(forwarded)
		}
		return forwarded, nil
	})
	return sent, t, err
}

func viewOnce(m *wire.Message) bool {
	for _, wrapped := range []*wire.Message{m, m.GetEphemeralMessage().GetMessage()} {
		if wrapped.GetViewOnceMessage() != nil || wrapped.GetViewOnceMessageV2() != nil || wrapped.GetViewOnceMessageV2Extension() != nil {
			return true
		}
	}
	inner := media.Unwrap(m)
	return inner.GetImageMessage().GetViewOnce() || inner.GetVideoMessage().GetViewOnce() || inner.GetAudioMessage().GetViewOnce()
}

type NoOption struct {
	Choice string
}

func (n NoOption) Error() string {
	return fmt.Sprintf("%v: %q", ErrNoOption, n.Choice)
}

func (n NoOption) Is(target error) bool {
	return target == ErrNoOption
}
