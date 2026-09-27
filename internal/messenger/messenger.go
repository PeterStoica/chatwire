package messenger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
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
	ErrRestricted     = errors.New("messenger: WhatsApp is limiting messages to new contacts")
)

const (
	restrictedFor = 24 * time.Hour
	groupsFresh   = 5 * time.Minute
)

type cachedGroup struct {
	group groups.Group
	at    time.Time
}

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
	limited  time.Time
	listed   []groups.Group
	listedAt time.Time
	known    map[node.JID]cachedGroup
	failures int
	retryAt  time.Time
	outdated func()
	saving   sync.Mutex
	current  int
}

type HistorySync struct {
	Started bool
	Percent uint32
}

func New(link linkflow.Config, httpClient *http.Client, persist func(client.State) error, messages *store.Store) *Messenger {
	return &Messenger{link: link, http: httpClient, persist: persist, store: messages, ready: make(chan struct{}), failed: make(chan struct{}), known: map[node.JID]cachedGroup{}}
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
	r := newRetries()
	for ctx.Err() == nil {
		m.mu.Lock()
		state := *m.state
		m.mu.Unlock()
		lids, _ := m.store.LIDs(ctx)
		start := m.link.Now()
		c, err := client.Connect(ctx, client.Config{LIDs: lids, Link: m.link, HTTP: m.http, Persist: m.saver(), Receive: m.received, History: m.history, Receipt: m.receipt, Sent: m.sentMessage, Seen: m.seen, TokenOf: m.tokenOf, Tokens: m.tokens, Changed: m.groupChanged, Problem: problem, AppState: m}, state)
		if err == nil {
			err = m.online(ctx, c)
		} else {
			m.mu.Lock()
			m.lastErr = err
			close(m.failed)
			m.failed = make(chan struct{})
			m.mu.Unlock()
		}
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, linkflow.ErrLoggedOut) {
			m.loggedOut(err)
			return
		}
		delay, refresh := r.next(err, m.link.Now().Sub(start))
		m.mu.Lock()
		m.failures, m.retryAt = r.failures, m.link.Now().Add(delay)
		outdated := m.outdated
		m.mu.Unlock()
		if refresh && outdated != nil {
			outdated()
		}
		if delay == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (m *Messenger) online(ctx context.Context, c *client.Client) error {
	m.mu.Lock()
	m.client, m.lastErr, m.failures, m.retryAt = c, nil, 0, time.Time{}
	m.listed, m.known = nil, map[node.JID]cachedGroup{}
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
	return c.Err()
}

func (m *Messenger) WhenOutdated(refresh func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outdated = refresh
}

func (m *Messenger) WhenLoggedOut(fn func(error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gone = fn
}

func (m *Messenger) loggedOut(err error) {
	m.saving.Lock()
	defer m.saving.Unlock()
	m.mu.Lock()
	m.state, m.lastErr = nil, err
	m.current++
	gone := m.gone
	m.mu.Unlock()
	if gone != nil {
		gone(err)
	}
}

func (m *Messenger) saver() func(client.State) error {
	m.mu.Lock()
	m.current++
	mine := m.current
	m.mu.Unlock()
	return func(state client.State) error {
		m.saving.Lock()
		defer m.saving.Unlock()
		m.mu.Lock()
		if mine != m.current || m.state == nil {
			m.mu.Unlock()
			return nil
		}
		m.state = &state
		m.mu.Unlock()
		return m.persist(state)
	}
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
	Failures  int
	Retry     time.Time
}

func (m *Messenger) Connection() Connection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Connection{Linked: m.state != nil, Connected: m.client != nil, Err: m.lastErr, StoreErr: m.storeErr, History: m.synced, Failures: m.failures, Retry: m.retryAt}
}

func (m *Messenger) Groups(ctx context.Context) ([]groups.Group, error) {
	now := m.link.Now()
	m.mu.Lock()
	cached, at := m.listed, m.listedAt
	m.mu.Unlock()
	if cached != nil && now.Sub(at) < groupsFresh {
		return slices.Clone(cached), nil
	}
	c, err := m.connected(ctx)
	if err != nil {
		return nil, err
	}
	listed, err := c.Groups(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.listed, m.listedAt = listed, now
	for _, g := range listed {
		m.known[g.JID] = cachedGroup{group: g, at: now}
	}
	m.mu.Unlock()
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
