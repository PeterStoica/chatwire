package messenger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (m *Messenger) SendText(ctx context.Context, to node.JID, text string, mentions ...node.JID) (string, error) {
	msg := &wire.Message{Conversation: new(text)}
	if len(mentions) > 0 {
		msg = &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new(text), ContextInfo: &wire.ContextInfo{MentionedJid: addresses(mentions)}}}
	}
	return m.send(ctx, to, func(*client.Client) (*wire.Message, error) { return msg, nil })
}

func (m *Messenger) SendPoll(ctx context.Context, to node.JID, question string, options []string, multiple bool) (string, error) {
	question, options, err := message.CheckPoll(question, options)
	if err != nil {
		return "", err
	}
	poll := message.NewPoll(question, options, multiple)
	return m.send(ctx, to, func(*client.Client) (*wire.Message, error) { return poll, nil })
}

func addresses(jids []node.JID) []string {
	out := make([]string, 0, len(jids))
	for _, j := range jids {
		out = append(out, j.WithoutDevice().String())
	}
	return out
}

func (m *Messenger) ChatOf(ctx context.Context, id string) (node.JID, error) {
	t, _, err := m.target(ctx, id)
	return t.Chat, err
}

func (m *Messenger) Reply(ctx context.Context, to node.JID, text, quotedID string, mentions ...node.JID) (string, node.JID, error) {
	quoted, _, err := m.target(ctx, quotedID)
	if err != nil {
		return "", node.JID{}, err
	}
	if to.Server == "" {
		to = quoted.Chat
	}
	reply := message.Reply(text, message.Quote{ID: quoted.ID, Author: quoted.Author, Chat: quoted.Chat, Message: quoted.Message}, to.WithoutDevice() != quoted.Chat.WithoutDevice())
	if len(mentions) > 0 {
		reply.ExtendedTextMessage.ContextInfo.MentionedJid = addresses(mentions)
	}
	id, err := m.send(ctx, to, func(*client.Client) (*wire.Message, error) { return reply, nil })
	return id, to, err
}

func (m *Messenger) SendFile(ctx context.Context, to node.JID, f File) (string, error) {
	return m.send(ctx, to, func(c *client.Client) (*wire.Message, error) { return fileMessage(ctx, c, f, m.link.Now()) })
}

func (m *Messenger) send(ctx context.Context, to node.JID, build func(*client.Client) (*wire.Message, error)) (string, error) {
	c, err := m.connected(ctx)
	if err != nil {
		return "", err
	}
	if err := m.pace(ctx, to); err != nil {
		return "", err
	}
	var group *groups.Group
	if to.Server == node.ServerGroup {
		found, err := m.group(ctx, c, to)
		if err != nil {
			return "", err
		}
		group = &found
	}
	msg, err := build(c)
	if err != nil {
		return "", err
	}
	if message.NeedsSecret(msg) {
		secret := make([]byte, message.SecretSize)
		if _, err := io.ReadFull(m.link.Random, secret); err != nil {
			return "", fmt.Errorf("messenger: message secret: %w", err)
		}
		msg = message.WithSecret(msg, secret)
	}
	var id string
	if group != nil {
		id, err = c.SendGroup(ctx, *group, msg)
	} else {
		id, err = c.Send(ctx, to, msg)
	}
	if err == nil {
		m.sent(ctx, to, id, msg)
	}
	if rejected := (client.Rejection{}); errors.As(err, &rejected) && rejected.Code == client.CodeRestricted {
		m.mu.Lock()
		m.limited = m.link.Now().Add(restrictedFor)
		m.mu.Unlock()
	}
	return id, err
}

func (m *Messenger) group(ctx context.Context, c *client.Client, jid node.JID) (groups.Group, error) {
	now := m.link.Now()
	m.mu.Lock()
	cached, ok := m.known[jid]
	m.mu.Unlock()
	if ok && now.Sub(cached.at) < groupsFresh {
		return cached.group, nil
	}
	g, found, err := c.Group(ctx, jid)
	if err != nil {
		return groups.Group{}, err
	}
	if !found {
		return groups.Group{}, fmt.Errorf("%w: %s", ErrNotMember, jid)
	}
	m.mu.Lock()
	m.known[jid] = cachedGroup{group: g, at: now}
	m.mu.Unlock()
	return g, nil
}

func (m *Messenger) groupChanged(group node.JID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.known, group)
	m.listed = nil
}

func (m *Messenger) MarkRead(ctx context.Context, messages []store.Message) error {
	c, err := m.connected(ctx)
	if err != nil {
		return err
	}
	type thread struct{ chat, sender node.JID }
	var order []thread
	ids := map[thread][]string{}
	for _, msg := range messages {
		if msg.FromMe || msg.Revoked {
			continue
		}
		t := thread{chat: msg.Chat, sender: msg.Author}
		if _, seen := ids[t]; !seen {
			order = append(order, t)
		}
		ids[t] = append(ids[t], msg.ID)
	}
	var (
		failures []error
		seen     []node.JID
	)
	for _, t := range order {
		if err := c.MarkRead(ctx, t.chat, t.sender, ids[t]); err != nil {
			failures = append(failures, err)
			continue
		}
		if !slices.Contains(seen, t.chat) {
			seen = append(seen, t.chat)
		}
	}
	m.keep(ctx, func(ctx context.Context) error { return m.store.Apply(ctx, store.Changes{Seen: seen}) })
	return errors.Join(failures...)
}

type File struct {
	Name    string
	Data    []byte
	Caption string
}

func fileMessage(ctx context.Context, c *client.Client, f File, now time.Time) (*wire.Message, error) {
	mimetype := media.Sniff(f.Name, f.Data)
	kind := media.KindOf(mimetype)
	up, err := c.Upload(ctx, kind, f.Data)
	if err != nil {
		return nil, err
	}
	stamp := now.Unix()
	var caption *string
	if f.Caption != "" {
		caption = new(f.Caption)
	}
	switch kind {
	case media.Image:
		image := &wire.Message_ImageMessage{
			Url: new(up.URL), DirectPath: new(up.DirectPath), MediaKey: up.MediaKey, Mimetype: new(mimetype),
			FileEncSha256: up.FileEncSHA256, FileSha256: up.FileSHA256, FileLength: new(up.FileLength), MediaKeyTimestamp: new(stamp), Caption: caption,
		}
		if picture, ok := media.Describe(f.Data); ok {
			image.Width, image.Height, image.JpegThumbnail = new(picture.Width), new(picture.Height), picture.Thumbnail
		}
		return &wire.Message{ImageMessage: image}, nil
	case media.Video:
		return &wire.Message{VideoMessage: &wire.Message_VideoMessage{
			Url: new(up.URL), DirectPath: new(up.DirectPath), MediaKey: up.MediaKey, Mimetype: new(mimetype),
			FileEncSha256: up.FileEncSHA256, FileSha256: up.FileSHA256, FileLength: new(up.FileLength), MediaKeyTimestamp: new(stamp), Caption: caption,
		}}, nil
	case media.Voice, media.Audio:
		return &wire.Message{AudioMessage: &wire.Message_AudioMessage{
			Url: new(up.URL), DirectPath: new(up.DirectPath), MediaKey: up.MediaKey, Mimetype: new(mimetype),
			FileEncSha256: up.FileEncSHA256, FileSha256: up.FileSHA256, FileLength: new(up.FileLength), MediaKeyTimestamp: new(stamp), Ptt: new(kind == media.Voice),
		}}, nil
	default:
		document := &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{
			Url: new(up.URL), DirectPath: new(up.DirectPath), MediaKey: up.MediaKey, Mimetype: new(mimetype),
			FileEncSha256: up.FileEncSHA256, FileSha256: up.FileSHA256, FileLength: new(up.FileLength), MediaKeyTimestamp: new(stamp),
			FileName: new(f.Name), Title: new(f.Name), Caption: caption,
		}}
		if caption != nil {
			return &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: document}}, nil
		}
		return document, nil
	}
}

func (m *Messenger) pace(ctx context.Context, to node.JID) error {
	p, err := m.store.Pace(ctx, to.WithoutDevice(), m.link.Now())
	switch {
	case err != nil:
		return err
	case p.LastMinute >= MaxPerMinute:
		return fmt.Errorf("%w: %d messages in the last minute", ErrTooFast, p.LastMinute)
	case p.Known || to.Server == node.ServerGroup || m.Mine(to):
		return nil
	case p.NewChats >= MaxNewChats:
		return fmt.Errorf("%w: %d new chats in the last 24 hours", ErrNewChats, p.NewChats)
	}
	m.mu.Lock()
	limited := m.limited
	m.mu.Unlock()
	if now := m.link.Now(); now.Before(limited) {
		return fmt.Errorf("%w until %s", ErrRestricted, limited.Format(time.RFC3339))
	}
	return nil
}
