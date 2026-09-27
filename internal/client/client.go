package client

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/live"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/usync"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	MaxDownload = 100 << 20
	ackTimeout  = 30 * time.Second
	typeGroup   = "skmsg"
	jobQueue    = 16
	attrType    = "type"
	pingEvery   = 20 * time.Second
	pingWait    = 20 * time.Second
)

var (
	ErrClosed   = errors.New("client: connection closed")
	ErrRejected = errors.New("client: WhatsApp refused the message")
	ErrNoTarget = errors.New("client: the recipient has no devices")
)

type State struct {
	Linked        linkflow.Linked     `json:"linked"`
	PreKeys       map[uint32][32]byte `json:"prekeys"`
	NextPreKeyID  uint32              `json:"next_prekey_id"`
	Sessions      []SessionEntry      `json:"sessions,omitempty"`
	SenderKeys    []SenderKeyEntry    `json:"sender_keys,omitempty"`
	OwnSenderKeys []OwnSenderKeyEntry `json:"own_sender_keys,omitempty"`
	SyncKeys      []SyncKey           `json:"sync_keys,omitempty"`
}

type SyncKey struct {
	ID        []byte `json:"id"`
	Data      []byte `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type SessionEntry struct {
	Device node.JID       `json:"device"`
	Record jsontext.Value `json:"record"`
}

type SenderKeyEntry struct {
	Group  node.JID       `json:"group"`
	Sender node.JID       `json:"sender"`
	Record jsontext.Value `json:"record"`
}

type OwnSenderKeyEntry struct {
	Group   node.JID       `json:"group"`
	Record  jsontext.Value `json:"record"`
	Holders []node.JID     `json:"holders,omitempty"`
}

type Received struct {
	ID      string
	Chat    node.JID
	Author  node.JID
	Time    time.Time
	Name    string
	Edit    message.Edit
	Message *wire.Message
	Pairs   map[node.JID]node.JID
}

type Config struct {
	LIDs     map[node.JID]node.JID
	Link     linkflow.Config
	Persist  func(State) error
	Receive  func(Received)
	History  func(history.Chunk)
	Receipt  func(message.Receipt)
	AppState AppStateStore
	HTTP     *http.Client
}

type Client struct {
	cfg      Config
	identity device.Identity
	online   *linkflow.Online
	done     chan struct{}
	jobs     chan func(context.Context)
	life     context.Context
	stop     context.CancelFunc
	syncKeys map[string]appstate.Keys

	mu       sync.Mutex
	state    State
	sessions map[node.JID]*signal.Session
	lids     map[string]string
	groups   map[senderName]*signal.SenderKeys
	ownKeys  map[node.JID]*signal.SenderKey
	holders  map[node.JID]map[node.JID]bool
	acks     map[string]chan node.Node
	retries  map[string]chan mediaretry.Notification
	media    media.Conn
	err      error
}

func Connect(ctx context.Context, cfg Config, state State) (*Client, error) {
	identity, err := device.Restore(state.Linked.Identity)
	if err != nil {
		return nil, err
	}
	if state.PreKeys == nil {
		state.PreKeys = map[uint32][32]byte{}
	}
	if state.NextPreKeyID == 0 {
		state.NextPreKeyID = 1
	}
	online, err := linkflow.Open(ctx, cfg.Link, state.Linked)
	if err != nil {
		return nil, err
	}
	c := &Client{
		cfg: cfg, identity: identity, done: make(chan struct{}), state: state,
		sessions: map[node.JID]*signal.Session{}, lids: map[string]string{}, groups: map[senderName]*signal.SenderKeys{}, acks: map[string]chan node.Node{}, retries: map[string]chan mediaretry.Notification{},
		ownKeys: map[node.JID]*signal.SenderKey{}, holders: map[node.JID]map[node.JID]bool{},
	}
	if err := c.restore(); err != nil {
		_ = online.Close()
		return nil, err
	}
	account := state.Linked.Account
	c.learnLocked(map[node.JID]node.JID{account.LID.WithoutDevice(): account.JID.WithoutDevice()})
	c.learnLocked(cfg.LIDs)
	c.online = online
	c.life, c.stop = context.WithCancel(context.WithoutCancel(ctx))
	c.jobs = make(chan func(context.Context), jobQueue)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		c.work()
	}()
	go c.loop(ctx, workerDone)
	if err := c.setUp(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	if cfg.Link.KeepAlive >= 0 {
		go c.keepAlive(ctx, cmp.Or(cfg.Link.KeepAlive, pingEvery))
	}
	return c, nil
}

func (c *Client) keepAlive(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.life.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
		}
		pingCtx, cancel := context.WithTimeout(ctx, pingWait)
		_, err := c.online.Session.Query(pingCtx, PingRequest())
		cancel()
		if err != nil && ctx.Err() == nil && c.life.Err() == nil {
			c.fail(fmt.Errorf("%w: no answer to a keepalive ping within %s", ErrClosed, pingWait))
			_ = c.online.Close()
			return
		}
	}
}

func Pong(ping node.Node) (node.Node, bool) {
	kind, _ := ping.Attr(attrType).Text()
	xmlns, _ := ping.Attr("xmlns").Text()
	if ping.Tag != "iq" || kind != "get" || xmlns != "urn:xmpp:ping" {
		return node.Node{}, false
	}
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "id", Value: ping.Attr("id")}, {Key: attrType, Value: node.Text("result")}, {Key: "to", Value: ping.Attr("from")},
	}}, true
}

func PingRequest() node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "id", Value: node.Value{}}, {Key: attrType, Value: node.Text("get")},
		{Key: "xmlns", Value: node.Text("w:p")}, {Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})},
	}}
}

func (c *Client) setUp(ctx context.Context) error {
	if err := c.ensurePreKeys(ctx); err != nil {
		return err
	}
	if _, err := c.online.Session.Query(ctx, ActiveRequest()); err != nil {
		return fmt.Errorf("client: go active: %w", err)
	}
	c.queueAppStateSync(nil)
	return nil
}

func (c *Client) ensurePreKeys(ctx context.Context) error {
	reply, err := c.online.Session.Query(ctx, prekeys.DigestRequest())
	if !prekeys.NeedsUpload(reply) {
		if err != nil {
			return fmt.Errorf("client: key digest: %w", err)
		}
		return nil
	}
	return c.uploadPreKeys(ctx)
}

func (c *Client) uploadPreKeys(ctx context.Context) error {
	c.mu.Lock()
	keys, next, err := prekeys.Generate(c.cfg.Link.Random, c.state.NextPreKeyID, prekeys.Batch)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	for _, k := range keys {
		c.state.PreKeys[k.ID] = k.Key.Seed()
	}
	c.state.NextPreKeyID = next
	err = c.cfg.Persist(c.snapshotLocked())
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("client: keep prekeys before uploading: %w", err)
	}
	reply, err := c.online.Session.Query(ctx, prekeys.Upload(c.identity.Registration(device.Props()), keys))
	if reply.Tag == "" {
		return fmt.Errorf("client: upload prekeys: %w", err)
	}
	return prekeys.Result(reply)
}

func (c *Client) Close() error {
	err := c.online.Close()
	<-c.done
	return err
}

func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Client) Done() <-chan struct{} {
	return c.done
}

func (c *Client) Self() node.JID {
	return c.state.Linked.Account.JID
}

func (c *Client) PreKey(id uint32) (curve.KeyPair, bool) {
	seed, ok := c.state.PreKeys[id]
	if !ok {
		return curve.KeyPair{}, false
	}
	pair, err := curve.NewKeyPair(bytes.NewReader(seed[:]))
	return pair, err == nil
}

func (c *Client) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return c.identity.SignedPreKey(), id == c.identity.Registration(device.Props()).SignedPreKey.ID
}

func (c *Client) mine(j node.JID) bool {
	return c.state.Linked.Account.Owns(j)
}

func (c *Client) loop(ctx context.Context, workerDone <-chan struct{}) {
	defer func() {
		c.stop()
		<-workerDone
		close(c.done)
	}()
	for n := range c.online.Session.Events() {
		switch n.Tag {
		case "message":
			c.receive(ctx, n)
		case "receipt":
			_ = c.online.Session.Send(ctx, live.Ack(n))
			if r, err := message.ParseReceipt(n); err == nil && c.cfg.Receipt != nil {
				c.cfg.Receipt(r)
			}
		case "notification":
			_ = c.online.Session.Send(ctx, live.Ack(n))
			c.notified(n)
		case "ack":
			c.acked(n)
		case "iq":
			if pong, ok := Pong(n); ok {
				_ = c.online.Session.Send(ctx, pong)
			}
		case "stream:error", "failure":
			c.fail(fmt.Errorf("%w: %s", linkflow.Classify(n, ErrClosed), n))
			_ = c.online.Close()
		}
	}
	c.fail(fmt.Errorf("%w: %w", ErrClosed, c.online.Session.Err()))
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, waiter := range c.acks {
		close(waiter)
		delete(c.acks, id)
	}
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
}

func (c *Client) acked(n node.Node) {
	id, _ := n.Attr("id").Text()
	c.mu.Lock()
	waiter, ok := c.acks[id]
	delete(c.acks, id)
	c.mu.Unlock()
	if ok {
		waiter <- n
	}
}

func (c *Client) receive(ctx context.Context, n node.Node) {
	in, err := message.ParseIncoming(c.mine, n)
	if err != nil {
		_ = c.online.Session.Send(ctx, live.Ack(n))
		return
	}
	deferred := false
	for _, enc := range pairwiseFirst(in.Encs) {
		plaintext, err := c.decrypt(in, enc)
		if errors.Is(err, errUnsupported) {
			continue
		}
		if err != nil {
			c.retry(ctx, in, enc)
			return
		}
		decoded, err := message.Decode(plaintext)
		if err != nil {
			continue
		}
		if distribution := decoded.GetSenderKeyDistributionMessage(); distribution != nil {
			c.distribute(in, distribution.GetAxolotlSenderKeyDistributionMessage())
		}
		if c.mine(in.Author) {
			deferred = c.fromOurPhone(in, decoded.GetProtocolMessage()) || deferred
		}
		if c.cfg.Receive != nil && hasContent(decoded) {
			c.cfg.Receive(Received{ID: in.ID, Chat: in.Chat, Author: in.Author, Time: in.Timestamp, Name: in.PushName, Edit: in.Edit, Message: decoded, Pairs: in.Pairs})
		}
	}
	c.keep()
	if !deferred {
		_ = c.online.Session.Send(ctx, message.DeliveryReceipt(c.mine, in))
	}
}

var errUnsupported = errors.New("client: unsupported encryption")

type senderName struct {
	group  node.JID
	sender node.JID
}

func pairwiseFirst(encs []message.Enc) []message.Enc {
	ordered := slices.Clone(encs)
	slices.SortStableFunc(ordered, func(a, b message.Enc) int {
		return cmp.Compare(boolRank(a.Type == typeGroup), boolRank(b.Type == typeGroup))
	})
	return ordered
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

func hasContent(m *wire.Message) bool {
	if p := m.GetProtocolMessage(); p.GetKey() != nil && p.Type != nil && (p.GetType() == wire.Message_ProtocolMessage_REVOKE || p.GetType() == wire.Message_ProtocolMessage_MESSAGE_EDIT) {
		return true
	}
	rest := proto.CloneOf(m)
	rest.SenderKeyDistributionMessage, rest.MessageContextInfo, rest.ProtocolMessage = nil, nil, nil
	return proto.Size(rest) > 0
}

func (c *Client) distribute(in message.Incoming, distribution []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name := senderName{group: in.Chat, sender: c.addressLocked(in.Author)}
	keys := c.groups[name]
	if keys == nil {
		keys = &signal.SenderKeys{}
		c.groups[name] = keys
	}
	_ = keys.Process(distribution)
}

func (c *Client) decrypt(in message.Incoming, enc message.Enc) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.learnLocked(in.Pairs)
	author := c.addressLocked(in.Author)
	switch enc.Type {
	case "pkmsg":
		session, plaintext, used, err := signal.Respond(c.cfg.Link.Random, c.identity.Signal(), c, c.sessions[author], enc.Ciphertext)
		if err != nil {
			return nil, err
		}
		c.sessions[author] = session
		if used != nil {
			delete(c.state.PreKeys, *used)
			_ = c.cfg.Persist(c.snapshotLocked())
		}
		return plaintext, nil
	case "msg":
		return c.sessions[author].Decrypt(c.cfg.Link.Random, enc.Ciphertext)
	case typeGroup:
		keys := c.groups[senderName{group: in.Chat, sender: author}]
		if keys == nil {
			return nil, signal.ErrNoSenderKey
		}
		return keys.Decrypt(enc.Ciphertext)
	default:
		return nil, errUnsupported
	}
}

func (c *Client) retry(ctx context.Context, in message.Incoming, enc message.Enc) {
	c.mu.Lock()
	keys, next, err := prekeys.Generate(c.cfg.Link.Random, c.state.NextPreKeyID, 1)
	if err == nil {
		c.state.PreKeys[keys[0].ID] = keys[0].Key.Seed()
		c.state.NextPreKeyID = next
		err = c.cfg.Persist(c.snapshotLocked())
	}
	c.mu.Unlock()
	if err != nil {
		return
	}
	receipt, err := message.RetryReceipt(c.mine, in, message.Retry{
		Count: enc.Retry + 1, Registration: c.identity.Registration(device.Props()), PreKey: keys[0], DeviceIdentity: c.state.Linked.Account.SignedIdentity,
	})
	if err == nil {
		_ = c.online.Session.Send(ctx, receipt)
	}
}

func (c *Client) Send(ctx context.Context, to node.JID, m *wire.Message) (string, error) {
	self := node.JID{User: c.Self().User, Server: c.Self().Server}
	users := []node.JID{to}
	if to != self {
		users = append(users, self)
	}
	reply, err := c.online.Session.Query(ctx, usync.DevicesRequest(c.online.Session.NewID(), usync.ContextMessage, users))
	if err != nil {
		return "", fmt.Errorf("client: devices: %w", err)
	}
	listed, err := usync.ParseDevices(reply)
	if err != nil {
		return "", err
	}
	var targets []node.JID
	for _, user := range listed {
		for _, d := range user.Devices {
			if d.JID != c.Self() {
				targets = append(targets, d.JID)
			}
		}
	}
	if len(targets) == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoTarget, to)
	}
	if err := c.startSessions(ctx, targets); err != nil {
		return "", err
	}
	parts, err := c.encrypt(to, self, targets, m)
	if err != nil {
		return "", err
	}
	id, err := message.NewID(c.cfg.Link.Now(), self, c.cfg.Link.Random)
	if err != nil {
		return "", err
	}
	c.keep()
	return id, c.deliver(ctx, message.Outgoing(id, to, m, parts, c.state.Linked.Account.SignedIdentity))
}

func (c *Client) startSessions(ctx context.Context, targets []node.JID) error {
	c.mu.Lock()
	var missing []node.JID
	for _, target := range targets {
		if c.sessions[c.addressLocked(target)] == nil {
			missing = append(missing, target)
		}
	}
	c.mu.Unlock()
	if len(missing) == 0 {
		return nil
	}
	reply, err := c.online.Session.Query(ctx, prekeys.FetchRequest(missing))
	if err != nil {
		return fmt.Errorf("client: key bundles: %w", err)
	}
	bundles, _, err := prekeys.ParseBundles(reply)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, b := range bundles {
		address := c.addressLocked(b.Device)
		session, err := signal.Initiate(c.cfg.Link.Random, c.identity.Signal(), c.sessions[address], b.Keys)
		if err != nil {
			return fmt.Errorf("client: session with %s: %w", b.Device, err)
		}
		c.sessions[address] = session
	}
	return nil
}

func (c *Client) encrypt(to, self node.JID, targets []node.JID, m *wire.Message) ([]message.Part, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	parts := make([]message.Part, 0, len(targets))
	for _, target := range targets {
		session := c.sessions[c.addressLocked(target)]
		if session == nil {
			continue
		}
		payload := m
		if target.User == self.User && target.Server == self.Server {
			payload = message.SentByUs(to, m)
		}
		padded, err := message.Encode(c.cfg.Link.Random, payload)
		if err != nil {
			return nil, err
		}
		ciphertext, err := session.Encrypt(padded)
		if err != nil {
			return nil, err
		}
		parts = append(parts, message.Part{Device: target, Ciphertext: ciphertext})
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: no device accepted a session", ErrNoTarget)
	}
	return parts, nil
}

func (c *Client) deliver(ctx context.Context, stanza node.Node) error {
	id, _ := stanza.Attr("id").Text()
	waiter := make(chan node.Node, 1)
	c.mu.Lock()
	c.acks[id] = waiter
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.acks, id)
		c.mu.Unlock()
	}()
	if err := c.online.Session.Send(ctx, stanza); err != nil {
		return err
	}
	timeout, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()
	select {
	case ack, open := <-waiter:
		if !open {
			return ErrClosed
		}
		if failure, _ := ack.Attr("error").Text(); failure != "" {
			return fmt.Errorf("%w: error %s", ErrRejected, failure)
		}
		return nil
	case <-timeout.Done():
		return fmt.Errorf("client: no acknowledgement for %s: %w", id, timeout.Err())
	}
}

func (c *Client) Groups(ctx context.Context) ([]groups.Group, error) {
	reply, err := c.online.Session.Query(ctx, groups.ParticipatingRequest())
	if err != nil {
		return nil, fmt.Errorf("client: groups: %w", err)
	}
	return groups.ParseParticipating(reply)
}

func (c *Client) SendGroup(ctx context.Context, g groups.Group, m *wire.Message) (string, error) {
	users := make([]node.JID, 0, len(g.Participants))
	for _, p := range g.Participants {
		users = append(users, p.JID.WithoutDevice())
	}
	reply, err := c.online.Session.Query(ctx, usync.DevicesRequest(c.online.Session.NewID(), usync.ContextMessage, users))
	if err != nil {
		return "", fmt.Errorf("client: group devices: %w", err)
	}
	listed, err := usync.ParseDevices(reply)
	if err != nil {
		return "", err
	}
	var members []node.JID
	for _, user := range listed {
		for _, d := range user.Devices {
			if d.JID != c.Self() {
				members = append(members, d.JID)
			}
		}
	}
	key, needing, err := c.senderKeyFor(g.JID, members)
	if err != nil {
		return "", err
	}
	parts, err := c.distributeKey(ctx, g.JID, key, needing)
	if err != nil {
		return "", err
	}
	id, err := message.NewID(c.cfg.Link.Now(), node.JID{User: c.Self().User, Server: c.Self().Server}, c.cfg.Link.Random)
	if err != nil {
		return "", err
	}
	ciphertext, err := c.encryptForGroup(key, m)
	if err != nil {
		return "", err
	}
	stanza := message.OutgoingGroup(id, g.JID, m, g.AddressingMode, parts, ciphertext, c.state.Linked.Account.SignedIdentity)
	if err := c.deliver(ctx, stanza); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, part := range parts {
		c.holders[g.JID][c.addressLocked(part.Device)] = true
	}
	return id, c.keepLocked()
}

func (c *Client) senderKeyFor(group node.JID, members []node.JID) (*signal.SenderKey, []node.JID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.ownKeys[group]
	if key == nil {
		created, err := signal.NewSenderKey(c.cfg.Link.Random)
		if err != nil {
			return nil, nil, err
		}
		key = created
		c.ownKeys[group], c.holders[group] = key, map[node.JID]bool{}
	}
	var needing []node.JID
	for _, member := range members {
		if !c.holders[group][c.addressLocked(member)] {
			needing = append(needing, member)
		}
	}
	return key, needing, nil
}

func (c *Client) distributeKey(ctx context.Context, group node.JID, key *signal.SenderKey, needing []node.JID) ([]message.Part, error) {
	if len(needing) == 0 {
		return nil, nil
	}
	if err := c.startSessions(ctx, needing); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	distribution, err := key.Distribution()
	if err != nil {
		return nil, err
	}
	padded, err := message.Encode(c.cfg.Link.Random, message.SenderKeyDistribution(group, distribution))
	if err != nil {
		return nil, err
	}
	parts := make([]message.Part, 0, len(needing))
	for _, device := range needing {
		session := c.sessions[c.addressLocked(device)]
		if session == nil {
			continue
		}
		ciphertext, err := session.Encrypt(padded)
		if err != nil {
			return nil, err
		}
		parts = append(parts, message.Part{Device: device, Ciphertext: ciphertext})
	}
	return parts, nil
}

func (c *Client) encryptForGroup(key *signal.SenderKey, m *wire.Message) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	padded, err := message.Encode(c.cfg.Link.Random, m)
	if err != nil {
		return nil, err
	}
	return key.Encrypt(c.cfg.Link.Random, padded)
}

func (c *Client) restore() error {
	for _, entry := range c.state.Sessions {
		session := &signal.Session{}
		if err := session.UnmarshalBinary(entry.Record); err != nil {
			return fmt.Errorf("client: session with %s: %w", entry.Device, err)
		}
		c.sessions[entry.Device] = session
	}
	for _, entry := range c.state.SenderKeys {
		keys := &signal.SenderKeys{}
		if err := keys.UnmarshalBinary(entry.Record); err != nil {
			return fmt.Errorf("client: sender key of %s in %s: %w", entry.Sender, entry.Group, err)
		}
		c.groups[senderName{group: entry.Group, sender: entry.Sender}] = keys
	}
	for _, entry := range c.state.OwnSenderKeys {
		key := &signal.SenderKey{}
		if err := key.UnmarshalBinary(entry.Record); err != nil {
			return fmt.Errorf("client: our sender key in %s: %w", entry.Group, err)
		}
		c.ownKeys[entry.Group], c.holders[entry.Group] = key, map[node.JID]bool{}
		for _, holder := range entry.Holders {
			c.holders[entry.Group][holder] = true
		}
	}
	return nil
}

func (c *Client) keepLocked() error {
	c.state.Sessions = nil
	for device, session := range c.sessions {
		record, err := session.MarshalBinary()
		if err != nil {
			return err
		}
		c.state.Sessions = append(c.state.Sessions, SessionEntry{Device: device, Record: record})
	}
	c.state.SenderKeys = nil
	for name, keys := range c.groups {
		record, err := keys.MarshalBinary()
		if err != nil {
			return err
		}
		c.state.SenderKeys = append(c.state.SenderKeys, SenderKeyEntry{Group: name.group, Sender: name.sender, Record: record})
	}
	c.state.OwnSenderKeys = nil
	for group, key := range c.ownKeys {
		record, err := key.MarshalBinary()
		if err != nil {
			return err
		}
		entry := OwnSenderKeyEntry{Group: group, Record: record}
		for holder := range c.holders[group] {
			entry.Holders = append(entry.Holders, holder)
		}
		c.state.OwnSenderKeys = append(c.state.OwnSenderKeys, entry)
	}
	return c.cfg.Persist(c.snapshotLocked())
}

func (c *Client) snapshotLocked() State {
	snapshot := c.state
	snapshot.PreKeys = maps.Clone(c.state.PreKeys)
	return snapshot
}

func (c *Client) keep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.keepLocked()
}

var (
	ErrDownload = errors.New("client: media download failed")
	ErrGone     = errors.New("client: the file is no longer on the media servers")
	ErrRetry    = errors.New("client: the phone could not upload the file again")
)

func (c *Client) Download(ctx context.Context, ref media.Reference) ([]byte, error) {
	conn, err := c.mediaConn(ctx)
	if err != nil {
		return nil, err
	}
	hosts := slices.Clone(conn.Hosts)
	slices.SortStableFunc(hosts, func(a, b media.Host) int { return cmp.Compare(boolRank(a.Fallback), boolRank(b.Fallback)) })
	failures := make([]error, 0, len(hosts))
	for _, host := range hosts {
		address, err := media.DownloadURL(host.Hostname, ref.DirectPath, ref.FileEncSHA256, ref.Type)
		if err != nil {
			return nil, err
		}
		file, err := c.fetch(ctx, address)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		return media.Decrypt(ref.MediaKey, ref.Type, file, ref.FileEncSHA256, ref.FileSHA256)
	}
	return nil, fmt.Errorf("%w: %w", ErrDownload, errors.Join(failures...))
}

func (c *Client) mediaConn(ctx context.Context) (media.Conn, error) {
	now := c.cfg.Link.Now()
	c.mu.Lock()
	cached := c.media
	c.mu.Unlock()
	if cached.Fresh(now) {
		return cached, nil
	}
	reply, err := c.online.Session.Query(ctx, media.ConnRequest())
	if err != nil {
		return media.Conn{}, fmt.Errorf("client: media hosts: %w", err)
	}
	conn, err := media.ParseConn(reply, now)
	if err != nil {
		return media.Conn{}, err
	}
	c.mu.Lock()
	c.media = conn
	c.mu.Unlock()
	return conn, nil
}

func (c *Client) fetch(ctx context.Context, address string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Origin", dial.Origin)
	response, err := c.cfg.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusGone:
		return nil, fmt.Errorf("%w: %s answered %d", ErrGone, request.URL.Host, response.StatusCode)
	default:
		return nil, fmt.Errorf("%s answered %d", request.URL.Host, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxDownload {
		return nil, fmt.Errorf("%s sent more than %d bytes", request.URL.Host, MaxDownload)
	}
	return body, nil
}

func ActiveRequest() node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "id", Value: node.Value{}}, {Key: attrType, Value: node.Text("set")},
			{Key: "xmlns", Value: node.Text("passive")}, {Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})},
		},
		Children: []node.Node{{Tag: "active"}},
	}
}
