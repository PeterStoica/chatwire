package live

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PeterStoica/chatwire/internal/handshake"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/queue"
)

var ErrClosed = errors.New("live: session closed")

type IDs struct {
	prefix  string
	counter atomic.Uint64
}

func NewIDs(random io.Reader) (*IDs, error) {
	var raw [4]byte
	if _, err := io.ReadFull(random, raw[:]); err != nil {
		return nil, fmt.Errorf("live: stanza id prefix: %w", err)
	}
	return &IDs{prefix: fmt.Sprintf("%d.%d-", binary.BigEndian.Uint16(raw[:2]), binary.BigEndian.Uint16(raw[2:]))}, nil
}

func (ids *IDs) Next() string {
	return ids.prefix + strconv.FormatUint(ids.counter.Add(1), 10)
}

type Session struct {
	conn       *handshake.Conn
	dictionary node.Dictionary
	trace      io.Writer
	ids        *IDs
	writing    sync.Mutex
	mu         sync.Mutex
	waiting    map[string]chan node.Node
	inbox      *queue.Queue[node.Node]
	events     chan node.Node
	done       chan struct{}
	err        error
}

func Start(ctx context.Context, conn *handshake.Conn, dictionary node.Dictionary, random io.Reader) (*Session, error) {
	ids, err := NewIDs(random)
	if err != nil {
		return nil, err
	}
	s := &Session{
		conn: conn, dictionary: dictionary, ids: ids,
		waiting: map[string]chan node.Node{}, inbox: queue.New[node.Node](), events: make(chan node.Node), done: make(chan struct{}),
	}
	if os.Getenv("CHATWIRE_TRACE") != "" {
		s.trace = os.Stderr
	}
	go s.read(ctx)
	go s.pump(ctx)
	return s, nil
}

func (s *Session) NewID() string {
	return s.ids.Next()
}

func (s *Session) Events() <-chan node.Node {
	return s.events
}

func (s *Session) Err() error {
	<-s.done
	return s.err
}

func (s *Session) read(ctx context.Context) {
	defer close(s.done)
	for {
		plaintext, err := s.conn.Read(ctx)
		if err != nil {
			s.err = err
			if s.trace != nil {
				_, _ = fmt.Fprintf(s.trace, "%s ended: %v\n", time.Now().Format("15:04:05.000"), err)
			}
			return
		}
		n, err := s.dictionary.Unmarshal(plaintext)
		if err != nil {
			s.err = err
			return
		}
		s.log("<-", n)
		if pong, ok := Pong(n); ok {
			_ = s.Send(ctx, pong)
			continue
		}
		if !s.deliver(n) {
			s.inbox.Push(n)
		}
	}
}

func (s *Session) pump(ctx context.Context) {
	defer close(s.events)
	hand := func() bool {
		for _, n := range s.inbox.Take() {
			select {
			case s.events <- n:
			case <-ctx.Done():
				return false
			}
		}
		return true
	}
	for hand() {
		select {
		case <-s.inbox.Ready():
		case <-s.done:
			hand()
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Session) deliver(n node.Node) bool {
	if n.Tag != "iq" {
		return false
	}
	kind, _ := n.Attr("type").Text()
	if kind != "result" && kind != "error" {
		return false
	}
	id, _ := n.Attr("id").Text()
	s.mu.Lock()
	waiter, ok := s.waiting[id]
	delete(s.waiting, id)
	s.mu.Unlock()
	if ok {
		waiter <- n
	}
	return ok
}

func (s *Session) Send(ctx context.Context, n node.Node) error {
	encoded, err := s.dictionary.Marshal(n)
	if err != nil {
		return err
	}
	s.log("->", n)
	s.writing.Lock()
	defer s.writing.Unlock()
	return s.conn.Write(ctx, encoded)
}

func (s *Session) Query(ctx context.Context, request node.Node) (node.Node, error) {
	id := s.ids.Next()
	request = request.With("id", node.Text(id))
	reply := make(chan node.Node, 1)
	s.mu.Lock()
	s.waiting[id] = reply
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.waiting, id)
		s.mu.Unlock()
	}()
	if err := s.Send(ctx, request); err != nil {
		return node.Node{}, err
	}
	select {
	case response := <-reply:
		if kind, _ := response.Attr("type").Text(); kind == "error" {
			return response, fmt.Errorf("live: iq %s failed: %s", id, response)
		}
		return response, nil
	case <-s.done:
		return node.Node{}, ErrClosed
	case <-ctx.Done():
		return node.Node{}, ctx.Err()
	}
}

func Pong(ping node.Node) (node.Node, bool) {
	if ping.Tag != "iq" {
		return node.Node{}, false
	}
	kind, _ := ping.Attr("type").Text()
	xmlns, _ := ping.Attr("xmlns").Text()
	if kind != "get" || xmlns != "urn:xmpp:ping" {
		return node.Node{}, false
	}
	var attrs []node.Attr
	if id := ping.Attr("id"); !id.IsZero() {
		attrs = append(attrs, node.Attr{Key: "id", Value: id})
	}
	return node.Node{Tag: "iq", Attrs: append(attrs, node.Attr{Key: "type", Value: node.Text("result")}, node.Attr{Key: "to", Value: ping.Attr("from")})}, true
}

type Refusal int

const (
	Unsupported  Refusal = 415
	Unparsable   Refusal = 487
	UnknownKind  Refusal = 488
	BadContent   Refusal = 491
	AlreadySeen  Refusal = 496
	HandlerCrash Refusal = 500
)

func Ack(n node.Node) node.Node {
	attrs := []node.Attr{{Key: "to", Value: n.Attr("from")}, {Key: "id", Value: n.Attr("id")}, {Key: "class", Value: node.Text(n.Tag)}}
	if kind := n.Attr("type"); !kind.IsZero() && n.Tag != "message" {
		attrs = append(attrs, node.Attr{Key: "type", Value: kind})
	}
	if participant := n.Attr("participant"); !participant.IsZero() && participant.String() != n.Attr("from").String() {
		attrs = append(attrs, node.Attr{Key: "participant", Value: participant})
	}
	return node.Node{Tag: "ack", Attrs: attrs}
}

func Nack(n node.Node, why Refusal) node.Node {
	attrs := []node.Attr{{Key: "to", Value: n.Attr("from")}, {Key: "id", Value: n.Attr("id")}, {Key: "class", Value: node.Text(n.Tag)}}
	if kind := n.Attr("type"); !kind.IsZero() {
		attrs = append(attrs, node.Attr{Key: "type", Value: kind})
	}
	if participant := n.Attr("participant"); !participant.IsZero() {
		attrs = append(attrs, node.Attr{Key: "participant", Value: participant})
	}
	attrs = append(attrs, node.Attr{Key: "error", Value: node.Text(strconv.Itoa(int(why)))})
	return node.Node{Tag: "ack", Attrs: attrs}
}

func (s *Session) log(direction string, n node.Node) {
	if s.trace == nil {
		return
	}
	text := n.String()
	if len(text) > 2000 {
		text = text[:2000] + "...\n"
	}
	_, _ = fmt.Fprintf(s.trace, "%s %s %s", time.Now().Format("15:04:05.000"), direction, text)
}
