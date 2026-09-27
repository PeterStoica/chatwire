package messenger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	MaxPerMinute = 20
	MaxNewChats  = 10
	firstRetry   = time.Second
	maxRetry     = time.Minute
	storeTime    = 10 * time.Second
)

var (
	ErrNotLinked      = errors.New("messenger: WhatsApp is not linked")
	ErrUnknownMessage = errors.New("messenger: no such message")
	ErrNoMedia        = errors.New("messenger: the message has no downloadable media")
	ErrNotMember      = errors.New("messenger: the user is not in that group")
	ErrDeleted        = errors.New("messenger: that message was deleted")
	ErrTooFast        = errors.New("messenger: sending too fast")
	ErrNewChats       = errors.New("messenger: too many new chats started")
)

type Messenger struct {
	link    linkflow.Config
	http    *http.Client
	persist func(client.State) error
	store   *store.Store

	mu       sync.Mutex
	state    *client.State
	client   *client.Client
	ready    chan struct{}
	failed   chan struct{}
	lastErr  error
	storeErr error
	cancel   context.CancelFunc
	running  chan struct{}
	gone     func(error)
	synced   HistorySync
}

type HistorySync struct {
	Started bool
	Percent uint32
}

func New(link linkflow.Config, httpClient *http.Client, persist func(client.State) error, messages *store.Store) *Messenger {
	return &Messenger{link: link, http: httpClient, persist: persist, store: messages, ready: make(chan struct{}), failed: make(chan struct{})}
}

func (m *Messenger) Start(parent context.Context, state client.State) {
	m.Close()
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	running := make(chan struct{})
	m.mu.Lock()
	m.state, m.cancel, m.running, m.lastErr = &state, cancel, running, nil
	m.mu.Unlock()
	go func() {
		defer close(running)
		m.keepConnected(ctx)
	}()
}

func (m *Messenger) keepConnected(ctx context.Context) {
	wait := firstRetry
	for ctx.Err() == nil {
		m.mu.Lock()
		state := *m.state
		m.mu.Unlock()
		lids, _ := m.store.LIDs(ctx)
		c, err := client.Connect(ctx, client.Config{LIDs: lids, Link: m.link, HTTP: m.http, Persist: m.save, Receive: m.received, History: m.history, Receipt: m.receipt, Sent: m.sentMessage, Seen: m.seen, Problem: problem, AppState: m}, state)
		if errors.Is(err, linkflow.ErrLoggedOut) {
			m.loggedOut(err)
			return
		}
		if err != nil {
			m.mu.Lock()
			m.lastErr = err
			close(m.failed)
			m.failed = make(chan struct{})
			m.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = min(2*wait, maxRetry)
			continue
		}
		wait = firstRetry
		m.mu.Lock()
		m.client, m.lastErr = c, nil
		close(m.ready)
		m.mu.Unlock()
		_, _ = m.Groups(ctx)
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-c.Done():
			m.setErr(c.Err())
		}
		m.mu.Lock()
		m.client, m.ready = nil, make(chan struct{})
		m.mu.Unlock()
		if err := c.Err(); errors.Is(err, linkflow.ErrLoggedOut) {
			m.loggedOut(err)
			return
		}
	}
}

func (m *Messenger) WhenLoggedOut(fn func(error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gone = fn
}

func (m *Messenger) loggedOut(err error) {
	m.mu.Lock()
	m.state, m.lastErr = nil, err
	gone := m.gone
	m.mu.Unlock()
	if gone != nil {
		gone(err)
	}
}

func (m *Messenger) save(state client.State) error {
	m.mu.Lock()
	m.state = &state
	m.mu.Unlock()
	return m.persist(state)
}

func (m *Messenger) keep(ctx context.Context, write func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTime)
	defer cancel()
	err := write(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeErr = err
}

func (m *Messenger) sent(ctx context.Context, chat node.JID, id string, msg *wire.Message) {
	self, _ := m.Self()
	m.dispatch(ctx, client.Received{ID: id, Chat: chat, Author: self, Time: m.link.Now(), Edit: message.EditOf(msg), Message: msg})
}

func (m *Messenger) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastErr = err
}

func (m *Messenger) connected(ctx context.Context) (*client.Client, error) {
	m.mu.Lock()
	c, ready, failed, linked := m.client, m.ready, m.failed, m.state != nil
	m.mu.Unlock()
	if !linked {
		return nil, ErrNotLinked
	}
	if c != nil {
		return c, nil
	}
	select {
	case <-ready:
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.client == nil {
			return nil, fmt.Errorf("messenger: connection to WhatsApp dropped: %w", m.lastErr)
		}
		return m.client, nil
	case <-failed:
		m.mu.Lock()
		defer m.mu.Unlock()
		return nil, fmt.Errorf("messenger: cannot connect to WhatsApp: %w", m.lastErr)
	case <-ctx.Done():
		return nil, fmt.Errorf("messenger: not connected to WhatsApp yet: %w", ctx.Err())
	}
}

func (m *Messenger) Self() (node.JID, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return node.JID{}, false
	}
	account := m.state.Linked.Account.JID
	return account.WithoutDevice(), true
}

func (m *Messenger) Mine(j node.JID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return false
	}
	return m.state.Linked.Account.Owns(j)
}

type Connection struct {
	Linked    bool
	Connected bool
	Err       error
	StoreErr  error
	History   HistorySync
}

func (m *Messenger) Connection() Connection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Connection{Linked: m.state != nil, Connected: m.client != nil, Err: m.lastErr, StoreErr: m.storeErr, History: m.synced}
}

func (m *Messenger) Groups(ctx context.Context) ([]groups.Group, error) {
	c, err := m.connected(ctx)
	if err != nil {
		return nil, err
	}
	listed, err := c.Groups(ctx)
	if err != nil {
		return nil, err
	}
	chats := make([]store.Chat, 0, len(listed))
	for _, g := range listed {
		chats = append(chats, store.Chat{JID: g.JID, Name: g.Subject})
	}
	m.keep(ctx, func(ctx context.Context) error { return m.store.Apply(ctx, store.Changes{Chats: chats}) })
	return listed, nil
}

func (m *Messenger) Media(ctx context.Context, id string) (media.Reference, []byte, error) {
	found, ok, err := m.store.Message(ctx, id)
	if err != nil {
		return media.Reference{}, nil, err
	}
	if !ok {
		return media.Reference{}, nil, fmt.Errorf("%w: %s", ErrUnknownMessage, id)
	}
	ref, ok := media.ReferenceOf(found.Message)
	if !ok {
		return media.Reference{}, nil, fmt.Errorf("%w: %s", ErrNoMedia, id)
	}
	c, err := m.connected(ctx)
	if err != nil {
		return ref, nil, err
	}
	data, err := c.Download(ctx, ref)
	if !errors.Is(err, client.ErrGone) {
		return ref, data, err
	}
	path, err := c.RetryMedia(ctx, ref, found.ID, mediaretry.Target{Chat: found.Chat, FromMe: found.FromMe, Participant: found.Author})
	if err != nil {
		return ref, nil, err
	}
	found.Message = media.WithDirectPath(found.Message, path)
	m.keep(ctx, func(ctx context.Context) error {
		return m.store.Apply(ctx, store.Changes{Messages: []store.Message{found}})
	})
	ref.DirectPath = path
	data, err = c.Download(ctx, ref)
	return ref, data, err
}

func (m *Messenger) Messages(ctx context.Context, q store.Query) ([]store.Message, error) {
	return m.store.Messages(ctx, q)
}

func (m *Messenger) Chats(ctx context.Context, limit int) ([]store.Chat, error) {
	return m.store.Chats(ctx, limit)
}

func (m *Messenger) Names(ctx context.Context) (map[node.JID]store.Name, error) {
	return m.store.Names(ctx)
}

func (m *Messenger) LIDs(ctx context.Context) (map[node.JID]node.JID, error) {
	return m.store.LIDs(ctx)
}

func (m *Messenger) Close() {
	m.mu.Lock()
	cancel, running := m.cancel, m.running
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-running
	}
}

func problem(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s %v\n", time.Now().Format(time.DateTime), err)
}
