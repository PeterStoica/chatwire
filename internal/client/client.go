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
	"math/rand/v2"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/limits"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/live"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/queue"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/usync"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	MaxDownload  = 100 << 20
	ackTimeout   = 30 * time.Second
	typeGroup    = "skmsg"
	attrType     = "type"
	pingEvery    = 20 * time.Second
	pingWait     = 10 * time.Second
	maxPingCheck = 5 * time.Second
	deadAfter    = 75 * time.Second
	maxUnacked   = 2
	devicesFresh = time.Hour
)

var (
	ErrClosed      = errors.New("client: connection closed")
	ErrUnconfirmed = errors.New("client: WhatsApp did not confirm the message")
	ErrRejected    = errors.New("client: WhatsApp refused the message")
	ErrNoTarget    = errors.New("client: the recipient has no devices")
)

const (
	CodeMalformed   = 400
	CodeForbidden   = 403
	CodeUnsupported = 405
	CodeStaleGroup  = 421
	CodeRestricted  = 463
	CodeChatCap     = 475
	CodeInvalid     = 479
)

type cachedDevices struct {
	devices []node.JID
	at      time.Time
}

type Rejection struct {
	Code int
}

func (r Rejection) Error() string {
	return fmt.Sprintf("%v: error %d", ErrRejected, r.Code)
}

func (r Rejection) Is(target error) bool {
	return target == ErrRejected
}

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
	Sent     func(ctx context.Context, chat node.JID, id string) (*wire.Message, bool)
	Seen     func(ctx context.Context, chat node.JID, id string) bool
	TokenOf  func(ctx context.Context, contact node.JID) privacy.Token
	Tokens   func([]privacy.Token)
	Changed  func(group node.JID)
	Limits   func(limits.Notice)
	Problem  func(error)
	AppState AppStateStore
	HTTP     *http.Client
}

type Client struct {
	cfg      Config
	identity device.Identity
	online   *linkflow.Online
	done     chan struct{}
	jobs     *queue.Queue[func(context.Context)]
	life     context.Context
	stop     context.CancelFunc
	syncKeys map[string]appstate.Keys

	mu       sync.Mutex
	state    State
	sessions map[address]*signal.Session
	lids     map[string]string

	recent       map[string]sentMessage
	recentOrder  []string
	resends      map[string]int
	recreated    map[address]time.Time
	given        map[node.JID]time.Time
	askedKeys    map[string]time.Time
	knownDevices map[node.JID]cachedDevices
	awaiting     map[string]*wire.MessageKey
	unacked      int
	groups       map[senderName]*signal.SenderKeys
	ownKeys      map[node.JID]*signal.SenderKey
	holders      map[node.JID]map[address]bool
	acks         map[string]chan node.Node
	retries      map[string]chan mediaretry.Notification
	media        media.Conn
	err          error
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
		sessions: map[address]*signal.Session{}, lids: map[string]string{}, groups: map[senderName]*signal.SenderKeys{}, acks: map[string]chan node.Node{}, retries: map[string]chan mediaretry.Notification{},
		ownKeys: map[node.JID]*signal.SenderKey{}, holders: map[node.JID]map[address]bool{},
		recent: map[string]sentMessage{}, resends: map[string]int{}, recreated: map[address]time.Time{}, given: map[node.JID]time.Time{}, askedKeys: map[string]time.Time{}, knownDevices: map[node.JID]cachedDevices{}, awaiting: map[string]*wire.MessageKey{},
	}
	account := state.Linked.Account
	pairs := map[node.JID]node.JID{account.LID.WithoutDevice(): account.JID.WithoutDevice()}
	maps.Copy(pairs, cfg.LIDs)
	c.learnLocked(pairs)
	if err := c.restore(); err != nil {
		_ = online.Close()
		return nil, err
	}
	c.online = online
	c.life, c.stop = context.WithCancel(context.WithoutCancel(ctx))
	c.jobs = queue.New[func(context.Context)]()
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
	check := min(every/4, maxPingCheck)
	ticker := time.NewTicker(check)
	defer ticker.Stop()
	wall := func() time.Time { return c.cfg.Link.Now().Round(0) }
	lastCheck, lastPong := wall(), wall()
	due := time.Now().Add(pingGap(every))
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
		now := wall()
		woke := now.Sub(lastCheck) > 3*check
		lastCheck = now
		if !woke && time.Now().Before(due) {
			continue
		}
		due = time.Now().Add(pingGap(every))
		pingCtx, cancel := context.WithTimeout(ctx, pingWait)
		_, err := c.online.Session.Query(pingCtx, PingRequest())
		cancel()
		lastCheck = wall()
		switch {
		case ctx.Err() != nil || c.life.Err() != nil:
			return
		case err == nil:
			lastPong = lastCheck
			continue
		case !woke && wall().Sub(lastPong) < deadAfter:
			continue
		}
		c.fail(fmt.Errorf("%w: no answer to keepalive pings since %s", ErrClosed, lastPong.Format(time.TimeOnly)))
		_ = c.online.Close()
		return
	}
}

func pingGap(every time.Duration) time.Duration {
	return every + rand.N(every/2+1)
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
		c.handle(ctx, n)
	}
	c.fail(fmt.Errorf("%w: %w", ErrClosed, c.online.Session.Err()))
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, waiter := range c.acks {
		close(waiter)
		delete(c.acks, id)
	}
}

func (c *Client) handle(ctx context.Context, n node.Node) {
	defer func() {
		if r := recover(); r != nil {
			c.problem(n.Tag+" "+n.Attr("id").String(), r)
			if n.Tag == "message" {
				_ = c.online.Session.Send(ctx, live.Nack(n, live.HandlerCrash))
			}
		}
	}()
	switch n.Tag {
	case "message":
		c.receive(ctx, n)
	case "receipt":
		_ = c.online.Session.Send(ctx, live.Ack(n))
		if req, err := message.ParseRetryRequest(n); err == nil {
			c.enqueue(func(ctx context.Context) { _ = c.resend(ctx, req) })
			return
		}
		if r, err := message.ParseReceipt(n); err == nil && c.cfg.Receipt != nil {
			c.cfg.Receipt(r)
		}
	case "notification":
		_ = c.online.Session.Send(ctx, live.Ack(n))
		c.notified(n)
	case "call":
		_ = c.online.Session.Send(ctx, callAnswer(n))
	case "status":
		_ = c.online.Session.Send(ctx, live.Nack(n, live.Unsupported))
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

func callAnswer(n node.Node) node.Node {
	if len(n.Children) == 0 {
		return live.Ack(n)
	}
	payload := n.Children[0]
	switch payload.Tag {
	case "offer", "accept", "reject", "enc_rekey":
		return node.Node{Tag: "receipt", Attrs: []node.Attr{{Key: "to", Value: n.Attr("from")}, {Key: "id", Value: n.Attr("id")}}, Children: []node.Node{{
			Tag: payload.Tag, Attrs: []node.Attr{{Key: "call-id", Value: payload.Attr("call-id")}, {Key: "call-creator", Value: payload.Attr("call-creator")}},
		}}}
	}
	return node.Node{Tag: "ack", Attrs: []node.Attr{
		{Key: "to", Value: n.Attr("from")}, {Key: "id", Value: n.Attr("id")}, {Key: "class", Value: node.Text("call")}, {Key: "type", Value: node.Text(payload.Tag)},
	}}
}

func (c *Client) problem(what string, r any) {
	if c.cfg.Problem != nil {
		c.cfg.Problem(fmt.Errorf("client: %s crashed: %v\n%s", what, r, debug.Stack()))
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
		_ = c.online.Session.Send(ctx, live.Nack(n, live.Unparsable))
		return
	}
	reply, ok := c.open(ctx, n, in)
	c.keep()
	if ok {
		_ = c.online.Session.Send(ctx, reply)
	}
}

func (c *Client) open(ctx context.Context, n node.Node, in message.Incoming) (node.Node, bool) {
	if len(in.Encs) == 0 {
		c.unavailable(n, in)
	}
	deferred, readable, unreadable := false, false, false
	for _, enc := range pairwiseFirst(in.Encs) {
		plaintext, err := c.decrypt(in, enc)
		switch {
		case errors.Is(err, errUnsupported):
			continue
		case errors.Is(err, signal.ErrDuplicate):
			if c.cfg.Seen != nil && c.cfg.Seen(ctx, in.Chat, in.ID) {
				return message.DeliveryReceipt(c.mine, in), true
			}
			return live.Nack(n, live.AlreadySeen), true
		case err != nil:
			c.expectResend(in)
			return c.retry(in, enc)
		}
		decoded, err := message.Decode(plaintext)
		if err != nil {
			unreadable = true
			continue
		}
		readable = true
		if distribution := decoded.GetSenderKeyDistributionMessage(); distribution != nil {
			c.distribute(in, distribution.GetAxolotlSenderKeyDistributionMessage())
		}
		if c.mine(in.Author) {
			deferred = c.fromOurPhone(in, decoded.GetProtocolMessage()) || deferred
		}
		if c.cfg.Receive != nil && hasContent(decoded) {
			c.readable(in.ID)
			c.cfg.Receive(Received{ID: in.ID, Chat: in.Chat, Author: in.Author, Time: in.Timestamp, Name: in.PushName, Edit: in.Edit, Message: decoded, Pairs: in.Pairs})
		}
	}
	switch {
	case unreadable && !readable:
		return live.Nack(n, live.BadContent), true
	case deferred:
		return node.Node{}, false
	}
	return message.DeliveryReceipt(c.mine, in), true
}

var errUnsupported = errors.New("client: unsupported encryption")

type senderName struct {
	group  node.JID
	sender address
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
	p := m.GetProtocolMessage()
	switch {
	case p.GetKey() != nil && p.Type != nil && (p.GetType() == wire.Message_ProtocolMessage_REVOKE || p.GetType() == wire.Message_ProtocolMessage_MESSAGE_EDIT):
		return true
	case p.GetType() == wire.Message_ProtocolMessage_EPHEMERAL_SETTING:
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

func (c *Client) retry(in message.Incoming, enc message.Enc) (node.Node, bool) {
	c.mu.Lock()
	keys, next, err := prekeys.Generate(c.cfg.Link.Random, c.state.NextPreKeyID, 1)
	if err == nil {
		c.state.PreKeys[keys[0].ID] = keys[0].Key.Seed()
		c.state.NextPreKeyID = next
		err = c.cfg.Persist(c.snapshotLocked())
	}
	c.mu.Unlock()
	if err != nil {
		return node.Node{}, false
	}
	receipt, err := message.RetryReceipt(c.mine, in, message.Retry{
		Count: enc.Retry + 1, Registration: c.identity.Registration(device.Props()), PreKey: keys[0], DeviceIdentity: c.state.Linked.Account.SignedIdentity,
	})
	return receipt, err == nil
}

func (c *Client) Send(ctx context.Context, to node.JID, m *wire.Message) (string, error) {
	return c.SendWithID(ctx, to, "", m)
}

func (c *Client) SendWithID(ctx context.Context, to node.JID, id string, m *wire.Message) (string, error) {
	toSelf := c.mine(to)
	theirs := func(d node.JID) bool { return !c.mine(d) }
	users := []node.JID{to}
	if !toSelf {
		users = append(users, c.ownDevice().WithoutDevice())
	}
	targets, err := c.devices(ctx, users)
	if err != nil {
		return "", err
	}
	if !toSelf && !slices.ContainsFunc(targets, theirs) {
		return "", fmt.Errorf("%w: %s", ErrNoTarget, to)
	}
	if err := c.startSessions(ctx, targets); err != nil {
		return "", err
	}
	parts, err := c.encrypt(to, targets, m)
	if err != nil {
		return "", err
	}
	if !toSelf && !slices.ContainsFunc(parts, func(p message.Part) bool { return theirs(p.Device) }) {
		return "", fmt.Errorf("%w: no session could be started with any device of %s", ErrNoTarget, to)
	}
	id, err = c.idFor(id)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.rememberLocked(id, to, m)
	c.mu.Unlock()
	c.keep()
	token, personal := c.tokenFor(ctx, to)
	c.mu.Lock()
	other := c.alternateLocked(to)
	c.mu.Unlock()
	stanza := message.KnownAs(message.Outgoing(id, to, m, parts, c.state.Linked.Account.SignedIdentity), other)
	if err := c.deliver(ctx, withToken(stanza, token, c.cfg.Link.Now())); err != nil {
		return id, err
	}
	if personal && m.GetProtocolMessage() == nil {
		c.giveToken(token)
	}
	return id, nil
}

func (c *Client) tokenFor(ctx context.Context, to node.JID) (privacy.Token, bool) {
	if c.cfg.TokenOf == nil || to.Server != node.ServerUser && to.Server != node.ServerLID || c.mine(to) {
		return privacy.Token{}, false
	}
	token := c.cfg.TokenOf(ctx, to.WithoutDevice())
	token.Contact = to.WithoutDevice()
	return token, true
}

func withToken(stanza node.Node, token privacy.Token, now time.Time) node.Node {
	if token.Usable(now) {
		stanza.Children = append(slices.Clone(stanza.Children), token.Node())
	}
	return stanza
}

func (c *Client) giveToken(token privacy.Token) {
	now := c.cfg.Link.Now()
	c.mu.Lock()
	if last := c.given[token.Contact]; last.After(token.Ours) {
		token.Ours = last
	}
	number, known := c.numberLocked(token.Contact)
	due := known && token.Due(now)
	if due {
		c.given[token.Contact] = now
	}
	c.mu.Unlock()
	if !due {
		return
	}
	c.enqueue(func(ctx context.Context) {
		if _, err := c.online.Session.Query(ctx, privacy.Give(number, now)); err != nil {
			c.mu.Lock()
			delete(c.given, token.Contact)
			c.mu.Unlock()
			return
		}
		if c.cfg.Tokens != nil {
			c.cfg.Tokens([]privacy.Token{{Contact: token.Contact, Ours: now}})
		}
	})
}

func (c *Client) devices(ctx context.Context, users []node.JID) ([]node.JID, error) {
	now := c.cfg.Link.Now()
	var all, missing []node.JID
	c.mu.Lock()
	for _, u := range users {
		if known, ok := c.knownDevices[u.WithoutDevice()]; ok && now.Sub(known.at) < devicesFresh {
			all = append(all, known.devices...)
		} else {
			missing = append(missing, u.WithoutDevice())
		}
	}
	c.mu.Unlock()
	if len(missing) > 0 {
		reply, err := c.online.Session.Query(ctx, usync.DevicesRequest(c.online.Session.NewID(), usync.ContextMessage, missing))
		if err != nil {
			return nil, fmt.Errorf("client: devices: %w", err)
		}
		listed, err := usync.ParseDevices(reply)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		for _, user := range listed {
			found := make([]node.JID, 0, len(user.Devices))
			for _, d := range user.Devices {
				found = append(found, d.JID)
			}
			if user.Err == nil {
				c.knownDevices[user.JID.WithoutDevice()] = cachedDevices{devices: found, at: now}
			}
			all = append(all, found...)
		}
		c.mu.Unlock()
	}
	var out []node.JID
	c.mu.Lock()
	defer c.mu.Unlock()
	self, me, seen := c.addressLocked(c.Self()), address(c.ownDevice()), map[address]bool{}
	for _, d := range all {
		if at := c.addressLocked(d); at != self && at != me && !seen[at] {
			seen[at] = true
			out = append(out, d)
		}
	}
	return out, nil
}

func (c *Client) forgetDevicesLocked(users ...node.JID) {
	for _, u := range users {
		delete(c.knownDevices, u.WithoutDevice())
		delete(c.knownDevices, node.JID(c.addressLocked(u.WithoutDevice())))
	}
}

func (c *Client) ownDevice() node.JID {
	if lid := c.state.Linked.Account.LID; lid.Server == node.ServerLID {
		return node.JID{User: lid.User, Device: c.Self().Device, Server: node.ServerLID}
	}
	return c.Self()
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
	return c.fetchSessions(ctx, missing)
}

func (c *Client) encrypt(to node.JID, targets []node.JID, m *wire.Message) ([]message.Part, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	parts, err := c.sealLocked(targets, func(target node.JID) ([]byte, error) {
		if c.mine(target) {
			return message.Encode(c.cfg.Link.Random, message.SentByUs(to, m))
		}
		return message.Encode(c.cfg.Link.Random, m)
	})
	if err == nil && len(parts) == 0 {
		err = fmt.Errorf("%w: no device accepted a session", ErrNoTarget)
	}
	return parts, err
}

func (c *Client) sealLocked(devices []node.JID, payload func(node.JID) ([]byte, error)) ([]message.Part, error) {
	parts := make([]message.Part, 0, len(devices))
	for _, device := range devices {
		session := c.sessions[c.addressLocked(device)]
		if session == nil {
			continue
		}
		padded, err := payload(device)
		if err != nil {
			return nil, err
		}
		ciphertext, err := session.Encrypt(padded)
		if err != nil {
			return nil, err
		}
		parts = append(parts, message.Part{Device: device, Ciphertext: ciphertext})
	}
	return parts, nil
}

func (c *Client) deliver(ctx context.Context, stanza node.Node) error {
	_, err := c.deliverAck(ctx, stanza)
	return err
}

func (c *Client) deliverAck(ctx context.Context, stanza node.Node) (node.Node, error) {
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
		if ctx.Err() == nil {
			c.broken(fmt.Errorf("%w: writing %s failed: %w", ErrClosed, id, err))
			return node.Node{}, fmt.Errorf("%w: %w: %w", ErrUnconfirmed, ErrClosed, err)
		}
		return node.Node{}, err
	}
	timeout, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()
	select {
	case ack, open := <-waiter:
		if !open {
			return node.Node{}, fmt.Errorf("%w: %w", ErrUnconfirmed, ErrClosed)
		}
		c.mu.Lock()
		c.unacked = 0
		c.mu.Unlock()
		if failure, _ := ack.Attr("error").Text(); failure != "" {
			code, err := strconv.Atoi(failure)
			if err != nil {
				return ack, fmt.Errorf("%w: error %q", ErrRejected, failure)
			}
			return ack, Rejection{Code: code}
		}
		return ack, nil
	case <-timeout.Done():
		if ctx.Err() == nil {
			c.mu.Lock()
			c.unacked++
			stuck := c.unacked >= maxUnacked
			c.mu.Unlock()
			if stuck {
				c.broken(fmt.Errorf("%w: WhatsApp acknowledged none of the last %d messages", ErrClosed, maxUnacked))
				return node.Node{}, fmt.Errorf("%w: %w: no acknowledgement for %s", ErrUnconfirmed, ErrClosed, id)
			}
		}
		return node.Node{}, fmt.Errorf("%w: no acknowledgement for %s: %w", ErrUnconfirmed, id, timeout.Err())
	}
}

func (c *Client) broken(err error) {
	c.fail(err)
	_ = c.online.Close()
}

func (c *Client) Groups(ctx context.Context) ([]groups.Group, error) {
	reply, err := c.online.Session.Query(ctx, groups.ParticipatingRequest())
	if err != nil {
		return nil, fmt.Errorf("client: groups: %w", err)
	}
	return groups.ParseParticipating(reply)
}

func (c *Client) LookUp(ctx context.Context, numbers []string) ([]usync.Contact, error) {
	reply, err := c.online.Session.Query(ctx, usync.ContactsRequest(c.online.Session.NewID(), numbers))
	if err != nil {
		return nil, fmt.Errorf("client: look up numbers: %w", err)
	}
	found, err := usync.ParseContacts(reply)
	if err != nil {
		return nil, err
	}
	pairs := map[node.JID]node.JID{}
	for _, f := range found {
		if f.OnWhatsApp && f.LID.Server == node.ServerLID && f.JID.Server == node.ServerUser {
			pairs[f.LID.WithoutDevice()] = f.JID.WithoutDevice()
		}
	}
	c.Learn(pairs)
	return found, nil
}

func (c *Client) Query(ctx context.Context, request node.Node) (node.Node, error) {
	return c.online.Session.Query(ctx, request)
}

func (c *Client) Group(ctx context.Context, jid node.JID) (groups.Group, bool, error) {
	reply, err := c.online.Session.Query(ctx, groups.InfoRequest(jid))
	if err != nil {
		return groups.Group{}, false, fmt.Errorf("client: group %s: %w", jid, err)
	}
	found, err := groups.ParseInfo(reply)
	if err != nil {
		return groups.Group{}, false, err
	}
	for _, g := range found {
		if g.JID == jid.WithoutDevice() {
			return g, true, nil
		}
	}
	return groups.Group{}, false, nil
}

func (c *Client) SendGroup(ctx context.Context, g groups.Group, m *wire.Message) (string, error) {
	return c.SendGroupWithID(ctx, g, "", m)
}

func (c *Client) idFor(id string) (string, error) {
	if id != "" {
		return id, nil
	}
	return message.NewID(c.cfg.Link.Now(), c.Self().WithoutDevice(), c.cfg.Link.Random)
}

func (c *Client) SendGroupWithID(ctx context.Context, g groups.Group, id string, m *wire.Message) (string, error) {
	users := make([]node.JID, 0, len(g.Participants))
	for _, p := range g.Participants {
		users = append(users, p.JID.WithoutDevice())
	}
	members, err := c.devices(ctx, users)
	if err != nil {
		return "", err
	}
	key, needing, err := c.senderKeyFor(g.JID, members)
	if err != nil {
		return "", err
	}
	parts, err := c.distributeKey(ctx, g.JID, key, needing)
	if err != nil {
		return "", err
	}
	id, err = c.idFor(id)
	if err != nil {
		return "", err
	}
	ciphertext, err := c.encryptForGroup(key, m)
	if err != nil {
		return "", err
	}
	phash := message.Phash(append(slices.Clone(members), c.ownDevice()))
	stanza := message.OutgoingGroup(id, g.JID, m, g.AddressingMode, phash, parts, ciphertext, c.state.Linked.Account.SignedIdentity)
	ack, err := c.deliverAck(ctx, stanza)
	if theirs, _ := ack.Attr("phash").Text(); theirs != "" && theirs != phash {
		c.mu.Lock()
		c.forgetDevicesLocked(users...)
		c.mu.Unlock()
	}
	if err != nil {
		return id, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, part := range parts {
		c.holders[g.JID][c.addressLocked(part.Device)] = true
	}
	c.rememberLocked(id, g.JID, m)
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
		c.ownKeys[group], c.holders[group] = key, map[address]bool{}
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
	return c.sealLocked(needing, func(node.JID) ([]byte, error) { return padded, nil })
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
		to := c.addressLocked(entry.Device)
		c.sessions[to] = c.sessions[to].Merge(session)
	}
	for _, entry := range c.state.SenderKeys {
		keys := &signal.SenderKeys{}
		if err := keys.UnmarshalBinary(entry.Record); err != nil {
			return fmt.Errorf("client: sender key of %s in %s: %w", entry.Sender, entry.Group, err)
		}
		name := senderName{group: entry.Group, sender: c.addressLocked(entry.Sender)}
		if c.groups[name] == nil {
			c.groups[name] = keys
		}
	}
	for _, entry := range c.state.OwnSenderKeys {
		key := &signal.SenderKey{}
		if err := key.UnmarshalBinary(entry.Record); err != nil {
			return fmt.Errorf("client: our sender key in %s: %w", entry.Group, err)
		}
		c.ownKeys[entry.Group], c.holders[entry.Group] = key, map[address]bool{}
		for _, holder := range entry.Holders {
			c.holders[entry.Group][c.addressLocked(holder)] = true
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
		c.state.Sessions = append(c.state.Sessions, SessionEntry{Device: node.JID(device), Record: record})
	}
	c.state.SenderKeys = nil
	for name, keys := range c.groups {
		record, err := keys.MarshalBinary()
		if err != nil {
			return err
		}
		c.state.SenderKeys = append(c.state.SenderKeys, SenderKeyEntry{Group: name.group, Sender: node.JID(name.sender), Record: record})
	}
	c.state.OwnSenderKeys = nil
	for group, key := range c.ownKeys {
		record, err := key.MarshalBinary()
		if err != nil {
			return err
		}
		entry := OwnSenderKeyEntry{Group: group, Record: record}
		for holder := range c.holders[group] {
			entry.Holders = append(entry.Holders, node.JID(holder))
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
		if err == nil {
			var plain []byte
			if plain, err = media.Decrypt(ref.MediaKey, ref.Type, file, ref.FileEncSHA256, ref.FileSHA256); err == nil {
				return plain, nil
			}
		}
		failures = append(failures, err)
	}
	failed := errors.Join(failures...)
	if len(failures) > 0 && !slices.ContainsFunc(failures, func(err error) bool {
		return !errors.Is(err, ErrGone) && !errors.Is(err, media.ErrHash) && !errors.Is(err, media.ErrMAC)
	}) {
		return nil, fmt.Errorf("%w: %w", ErrGone, failed)
	}
	return nil, fmt.Errorf("%w: %w", ErrDownload, failed)
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
	case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
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
