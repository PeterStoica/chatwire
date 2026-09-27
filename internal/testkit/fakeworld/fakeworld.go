package fakeworld

import (
	"context"
	"math/rand/v2"
	"runtime"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/cert"
	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signon"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeappstate"
	"github.com/PeterStoica/chatwire/internal/testkit/fakegroups"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
	"github.com/PeterStoica/chatwire/internal/testkit/fakephone"
	"github.com/PeterStoica/chatwire/internal/testkit/fakerelay"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeserver"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	refsPerPairDevice = 6
	pairingPingEvery  = 30 * time.Second
	pairingPongWithin = 12 * time.Second
)

type World struct {
	Authority  *cert.Authority
	Static     curve.KeyPair
	Dictionary node.Dictionary
	Phone      *fakephone.Phone
	Random     *rand.ChaCha8
	Screen     func() string

	mu      sync.Mutex
	dials   int
	scripts []Script
	shownQR []string
	codes   []string
	typing  chan string
}

type Script func(*Conn)

type Conn struct {
	ctx      context.Context
	world    *World
	session  fakeserver.Session
	incoming chan node.Node
	hungUp   chan struct{}
}

func New(seed byte) (*World, error) {
	random := rand.NewChaCha8([32]byte{seed})
	authority, err := cert.NewAuthority(random)
	if err != nil {
		return nil, err
	}
	static, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	account, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	dictionary, err := node.LoadDictionary()
	if err != nil {
		return nil, err
	}
	w := &World{
		Authority: authority, Static: static, Dictionary: dictionary, Random: random,
		Phone: &fakephone.Phone{
			Account: account, Random: random, KeyIndex: 2,
			JID: node.JID{User: "40700000000", Device: 17, Server: node.ServerUser},
			LID: node.JID{User: "987654321", Device: 17, Server: node.ServerLID},
		},
	}
	w.Screen = w.lastShownQR
	return w, nil
}

func (w *World) Version() signon.Version {
	return signon.Version{Primary: 2, Secondary: 3000, Tertiary: 1}
}

func (w *World) lastShownQR() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.shownQR) == 0 {
		return ""
	}
	return w.shownQR[len(w.shownQR)-1]
}

func (w *World) Script(scripts ...Script) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.scripts = scripts
}

func (w *World) ShowQR(data string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shownQR = append(w.shownQR, data)
}

func (w *World) ShowCode(code string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.codes = append(w.codes, code)
}

func (w *World) ShownQR() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.shownQR...)
}

func (w *World) ShownCodes() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.codes...)
}

func (w *World) Dials() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dials
}

func (w *World) Dial(ctx context.Context) (frame.MessageConn, error) {
	w.mu.Lock()
	index := w.dials
	w.dials++
	var script Script
	if index < len(w.scripts) {
		script = w.scripts[index]
	}
	w.mu.Unlock()
	clientSide, serverSide := frame.NewPipe()
	chain, err := w.Authority.Issue(w.Random, w.Static.Public(), cert.Validity{NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)})
	if err != nil {
		return nil, err
	}
	serverCtx := context.WithoutCancel(ctx)
	go func() {
		defer serverSide.Close()
		session, err := fakeserver.Respond(serverCtx, serverSide, fakeserver.Config{DictVersion: w.Dictionary.Version(), Static: w.Static, Chain: chain, Random: w.Random})
		if err != nil || script == nil {
			return
		}
		quit := make(chan struct{})
		defer close(quit)
		c := &Conn{ctx: serverCtx, world: w, session: session, incoming: make(chan node.Node), hungUp: make(chan struct{})}
		go c.read(serverCtx, quit)
		script(c)
	}()
	return clientSide, nil
}

func (c *Conn) read(ctx context.Context, quit <-chan struct{}) {
	defer close(c.hungUp)
	frames := make(chan node.Node)
	go func() {
		defer close(frames)
		for {
			raw, err := c.session.Conn.Read(ctx)
			if err != nil {
				return
			}
			n, err := c.world.Dictionary.Unmarshal(raw)
			if err != nil {
				panic(err)
			}
			select {
			case frames <- n:
			case <-quit:
				return
			}
		}
	}()
	var queue []node.Node
	for frames != nil || len(queue) > 0 {
		var (
			out  chan node.Node
			next node.Node
		)
		if len(queue) > 0 {
			out, next = c.incoming, queue[0]
		}
		select {
		case n, ok := <-frames:
			if !ok {
				frames = nil
				continue
			}
			queue = append(queue, n)
		case out <- next:
			queue = queue[1:]
		case <-quit:
			return
		}
	}
}

func (w *World) Type(code string) {
	w.mu.Lock()
	typing := w.typing
	w.mu.Unlock()
	if typing == nil {
		panic("nobody is waiting for a code")
	}
	typing <- code
}

func (w *World) awaitCode() chan string {
	typed := make(chan string, 1)
	w.mu.Lock()
	w.typing = typed
	w.mu.Unlock()
	return typed
}

func (c *Conn) Payload() []byte {
	return c.session.Payload
}

func (c *Conn) Send(n node.Node) {
	encoded, err := c.world.Dictionary.Marshal(n)
	if err != nil {
		panic(err)
	}
	if err := c.session.Conn.Write(c.ctx, encoded); err != nil {
		runtime.Goexit()
	}
}

func (c *Conn) Receive() node.Node {
	select {
	case n := <-c.incoming:
		return n
	case <-c.hungUp:
		runtime.Goexit()
		return node.Node{}
	}
}

func (c *Conn) WaitForHangUp() {
	for {
		select {
		case <-c.incoming:
		case <-c.hungUp:
			return
		}
	}
}

func (w *World) OfferPairing(c *Conn) {
	c.Send(fakephone.PairDevice("p1", refsPerPairDevice))
	if ack := c.Receive(); ack.Tag != "iq" {
		panic("expected an ack for pair-device, got " + ack.String())
	}
}

func (w *World) AnswerHello(c *Conn) node.Node {
	hello := c.Receive()
	response, err := w.Phone.ReceiveHello(hello)
	if err != nil {
		panic(err)
	}
	c.Send(response)
	return hello
}

func (w *World) EnterCode(c *Conn, code string) {
	notification, err := w.Phone.TypeCode(code)
	if err != nil {
		panic(err)
	}
	c.Send(notification)
	if ack := c.Receive(); ack.Tag != "ack" {
		panic("expected an ack for the notification, got " + ack.String())
	}
	finish := c.Receive()
	advSecret, companion, err := w.Phone.CompanionFinish(finish)
	if err != nil {
		panic(err)
	}
	c.Send(fakephone.Result(finish))
	w.FinishPairing(c, companion, advSecret)
}

func (w *World) FinishPairing(c *Conn, companion curve.PublicKey, advSecret []byte) {
	success, err := w.Phone.PairSuccess("s1", companion, advSecret)
	if err != nil {
		panic(err)
	}
	c.Send(success)
	reply := c.Receive()
	if err := w.Phone.VerifyDeviceSign(reply, companion); err != nil {
		panic(err)
	}
	c.Send(node.Node{Tag: "stream:error", Attrs: []node.Attr{{Key: "code", Value: node.Text("515")}}})
}

func (w *World) QRPairing(scanAfter time.Duration) Script {
	return func(c *Conn) {
		w.OfferPairing(c)
		if !c.idle(scanAfter) {
			return
		}
		scan, err := fakephone.ParseQR(w.Screen())
		if err != nil {
			panic(err)
		}
		w.FinishPairing(c, scan.Identity, scan.AdvSecret)
	}
}

func (w *World) Unscanned() Script {
	return func(c *Conn) {
		w.OfferPairing(c)
		c.idle(0)
	}
}

func (c *Conn) idle(d time.Duration) bool {
	var end <-chan time.Time
	if d > 0 {
		end = time.After(d)
	}
	ping := time.NewTicker(pairingPingEvery)
	defer ping.Stop()
	for n := 1; ; n++ {
		select {
		case <-end:
			return true
		case <-c.hungUp:
			return false
		case <-c.incoming:
		case <-ping.C:
			if !c.pinged("ping-" + strconv.Itoa(n)) {
				return false
			}
		}
	}
}

func (c *Conn) pinged(id string) bool {
	c.Send(node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: "type", Value: node.Text("get")},
		{Key: "id", Value: node.Text(id)}, {Key: "xmlns", Value: node.Text("urn:xmpp:ping")},
	}})
	deadline := time.After(pairingPongWithin)
	for {
		select {
		case n := <-c.incoming:
			kind, _ := n.Attr("type").Text()
			answer, _ := n.Attr("id").Text()
			if n.Tag == "iq" && kind == "result" && answer == id {
				return true
			}
		case <-deadline:
			return false
		case <-c.hungUp:
			return false
		}
	}
}

func (w *World) CodePairing() Script {
	return func(c *Conn) {
		w.OfferPairing(c)
		hello := c.Receive()
		response, err := w.Phone.ReceiveHello(hello)
		if err != nil {
			panic(err)
		}
		typed := w.awaitCode()
		c.Send(response)
		var code string
		select {
		case code = <-typed:
		case unexpected := <-c.incoming:
			panic("the client sent " + unexpected.String() + " before the code was typed")
		case <-c.hungUp:
			return
		}
		w.EnterCode(c, code)
	}
}

func (w *World) Login(result node.Node) Script {
	return func(c *Conn) {
		if !w.validLogin(c) {
			c.Send(Failure("bad-login-payload"))
			return
		}
		c.Send(result)
		c.WaitForHangUp()
	}
}

func (w *World) validLogin(c *Conn) bool {
	var payload wire.ClientPayload
	if err := proto.Unmarshal(c.Payload(), &payload); err != nil {
		panic(err)
	}
	return strconv.FormatUint(payload.GetUsername(), 10) == w.Phone.JID.User && payload.GetDevice() == uint32(w.Phone.JID.Device) &&
		payload.GetPassive() && payload.GetPull() && payload.DevicePairingData == nil
}

func Success() node.Node {
	return node.Node{Tag: "success"}
}

func Failure(reason string) node.Node {
	return node.Node{Tag: "failure", Attrs: []node.Attr{{Key: "reason", Value: node.Text(reason)}}}
}

type Server struct {
	Keys     *fakekeys.Server
	Devices  *fakeusync.Server
	PushName string
	Deliver  func(device node.JID, stanza node.Node)
	Inbox    chan node.Node
	Received func(node.Node)
	Override func(node.Node) (node.Node, bool)
	Drop     func(node.Node) bool
	Groups   *fakegroups.Server
	Members  func(group node.JID) []node.JID
	AppState *fakeappstate.Server
}

func (w *World) Serve(s *Server) Script {
	return func(c *Conn) {
		if !w.validLogin(c) {
			c.Send(Failure("bad-login-payload"))
			return
		}
		c.Send(Success())
		for {
			select {
			case n := <-c.incoming:
				w.answer(c, s, n)
			case n := <-s.Inbox:
				c.Send(n)
			case <-c.hungUp:
				return
			}
		}
	}
}

func (w *World) answerIQ(c *Conn, s *Server, n node.Node) {
	switch xmlns, _ := n.Attr("xmlns").Text(); xmlns {
	case "passive", "w:p", "privacy":
		c.Send(fakephone.Result(n))
	case "encrypt":
		c.Send(s.Keys.Handle(w.Phone.JID, n))
	case "usync":
		c.Send(s.Devices.Handle(n))
	case "w:m":
		reply := fakephone.Result(n, node.Node{Tag: "media_conn", Attrs: []node.Attr{
			{Key: "auth", Value: node.Text("fake-auth")}, {Key: "ttl", Value: node.Text("3600")}, {Key: "auth_ttl", Value: node.Text("3600")},
			{Key: "max_buckets", Value: node.Text("4")},
		}, Children: []node.Node{
			{Tag: "host", Attrs: []node.Attr{{Key: "hostname", Value: node.Text("mmg.whatsapp.net")}}},
			{Tag: "host", Attrs: []node.Attr{{Key: "hostname", Value: node.Text("media-fallback.fna.whatsapp.net")}, {Key: "type", Value: node.Text("fallback")}}},
		}})
		c.Send(reply)
	case "w:g2":
		known := s.Groups
		if known == nil {
			known = fakegroups.New()
		}
		c.Send(known.Handle(n))
	case "w:sync:app:state":
		if s.AppState == nil {
			c.Send(node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: n.Attr("id")}, {Key: "type", Value: node.Text("error")}}})
			return
		}
		reply, err := s.AppState.Handle(n)
		if err != nil {
			panic(err)
		}
		c.Send(reply)
	}
}

func (w *World) answer(c *Conn, s *Server, n node.Node) {
	if s.Received != nil {
		s.Received(n)
	}
	if s.Drop != nil && s.Drop(n) {
		return
	}
	if s.Override != nil {
		if reply, ok := s.Override(n); ok {
			c.Send(reply)
			return
		}
	}
	switch n.Tag {
	case "iq":
		w.answerIQ(c, s, n)
	case "message":
		deliveries := fakerelay.DeliverFrom(w.Phone.JID, w.Phone.LID, s.PushName, time.Now(), n)
		if to, _ := n.Attr("to").JID(); to.Server == node.ServerGroup {
			deliveries = fakerelay.DeliverGroup(w.Phone.JID, s.PushName, time.Now(), n, s.Members(to))
		}
		for device, stanza := range deliveries {
			s.Deliver(device, stanza)
		}
		ack := node.Node{Tag: "ack", Attrs: []node.Attr{{Key: "class", Value: node.Text("message")}, {Key: "id", Value: n.Attr("id")}, {Key: "t", Value: node.Text(strconv.FormatInt(time.Now().Unix(), 10))}}}
		if theirs, ours := n.Attr("phash").String(), w.phash(s, n); theirs != "" && ours != "" && theirs != ours {
			ack.Attrs = append(ack.Attrs, node.Attr{Key: "phash", Value: node.Text(ours)})
		}
		c.Send(ack)
	}
}

func (w *World) phash(s *Server, n node.Node) string {
	to, _ := n.Attr("to").JID()
	if to.Server != node.ServerGroup || s.Groups == nil || s.Devices == nil {
		return ""
	}
	g, ok := s.Groups.Group(to)
	if !ok {
		return ""
	}
	users := make([]node.JID, 0, len(g.Participants))
	for _, p := range g.Participants {
		users = append(users, p.JID)
	}
	var devices []node.JID
	for _, d := range s.Devices.DevicesOf(users...) {
		if d != w.Phone.JID {
			devices = append(devices, d)
		}
	}
	own := w.Phone.JID
	if w.Phone.LID.Server == node.ServerLID {
		own = node.JID{User: w.Phone.LID.User, Device: w.Phone.JID.Device, Server: node.ServerLID}
	}
	return message.Phash(append(devices, own))
}
