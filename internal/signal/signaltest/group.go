package signaltest

import (
	"crypto/rand"
	"fmt"
	mathrand "math/rand/v2"

	"github.com/PeterStoica/chatwire/internal/signal"
)

const (
	GroupMembers = 3
	GroupRounds  = 90
)

type Member interface {
	Rotate() ([]byte, error)
	Distribution() ([]byte, error)
	Encrypt(text string) ([]byte, error)
	Process(sender string, distribution []byte) error
	Decrypt(sender string, raw []byte) ([]byte, error)
}

type MemberFactory func(name string) Member

type ourMember struct {
	own      *signal.SenderKey
	received map[string]*signal.SenderKeys
}

func NewOurMember(string) Member {
	return &ourMember{received: map[string]*signal.SenderKeys{}}
}

func (m *ourMember) Rotate() ([]byte, error) {
	own, err := signal.NewSenderKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	m.own = own
	return own.Distribution()
}

func (m *ourMember) Distribution() ([]byte, error) {
	return m.own.Distribution()
}

func (m *ourMember) Encrypt(text string) ([]byte, error) {
	return m.own.Encrypt(rand.Reader, []byte(text))
}

func (m *ourMember) Process(sender string, distribution []byte) error {
	keys, ok := m.received[sender]
	if !ok {
		keys = &signal.SenderKeys{}
		m.received[sender] = keys
	}
	return keys.Process(distribution)
}

func (m *ourMember) Decrypt(sender string, raw []byte) ([]byte, error) {
	keys, ok := m.received[sender]
	if !ok {
		return nil, signal.ErrNoSenderKey
	}
	return keys.Decrypt(raw)
}

type groupWire struct {
	from  int
	bytes []byte
	text  string
}

type group struct {
	schedule *mathrand.Rand
	members  []Member
	held     [][]groupWire
	result   Result
}

func MemberName(i int) string {
	return fmt.Sprintf("member%d", i)
}

func GroupConverse(seed uint64, factories []MemberFactory) (Result, error) {
	g := &group{
		schedule: mathrand.New(mathrand.NewPCG(seed, 0x6770)),
		held:     make([][]groupWire, len(factories)),
		result:   Result{Stats: map[string]int{}},
	}
	for i, factory := range factories {
		g.members = append(g.members, factory(MemberName(i)))
	}
	for i := range g.members {
		if err := g.rotate(i); err != nil {
			return g.result, err
		}
	}
	for round := range GroupRounds {
		if err := g.round(round); err != nil {
			return g.result, fmt.Errorf("round %d: %w", round, err)
		}
	}
	for to := range g.members {
		if err := g.release(to); err != nil {
			return g.result, err
		}
	}
	return g.result, nil
}

func (g *group) rotate(from int) error {
	distribution, err := g.members[from].Rotate()
	if err != nil {
		return fmt.Errorf("%s rotates: %w", MemberName(from), err)
	}
	g.result.count("distributions", 1)
	g.result.Transcript = append(g.result.Transcript, fmt.Sprintf("distribution %x", distribution))
	for to, member := range g.members {
		if to == from {
			continue
		}
		if err := member.Process(MemberName(from), distribution); err != nil {
			return fmt.Errorf("%s processes %s's distribution: %w", MemberName(to), MemberName(from), err)
		}
	}
	return nil
}

func (g *group) round(round int) error {
	from := g.schedule.IntN(len(g.members) - 1)
	switch g.schedule.IntN(12) {
	case 0, 1:
		g.result.count("rotations", 1)
		return g.rotate(from)
	case 2:
		distribution, err := g.members[from].Distribution()
		if err != nil {
			return fmt.Errorf("%s distributes again: %w", MemberName(from), err)
		}
		g.result.count("mid-chain distributions", 1)
		g.result.Transcript = append(g.result.Transcript, fmt.Sprintf("distribution %x", distribution))
		return nil
	}
	batch := make([]groupWire, 1+g.schedule.IntN(5))
	for i := range batch {
		text := fmt.Sprintf("r%d-%d-%d", round, from, i)
		raw, err := g.members[from].Encrypt(text)
		if err != nil {
			return fmt.Errorf("%s encrypts: %w", MemberName(from), err)
		}
		g.result.Transcript = append(g.result.Transcript, fmt.Sprintf("send %x", raw))
		batch[i] = groupWire{from: from, bytes: raw, text: text}
	}
	for to := range g.members {
		if to == from {
			continue
		}
		if err := g.fanOut(to, batch); err != nil {
			return err
		}
	}
	return nil
}

func (g *group) fanOut(to int, batch []groupWire) error {
	order := append([]groupWire(nil), batch...)
	g.schedule.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	if to == g.offline() {
		g.result.count("queued for the offline member", len(order))
		g.held[to] = append(g.held[to], order...)
		return nil
	}
	if len(order) > 1 && g.schedule.IntN(3) == 0 {
		g.result.count("held back", 1)
		g.held[to] = append(g.held[to], order[len(order)-1])
		order = order[:len(order)-1]
	}
	for _, w := range order {
		if err := g.deliver(to, w); err != nil {
			return err
		}
	}
	if g.schedule.IntN(5) == 0 {
		return g.release(to)
	}
	return nil
}

func (g *group) offline() int {
	return len(g.members) - 1
}

func (g *group) deliver(to int, w groupWire) error {
	g.result.count("deliveries", 1)
	plaintext, err := g.members[to].Decrypt(MemberName(w.from), w.bytes)
	switch {
	case err != nil:
		g.result.count("undecryptable", 1)
		g.result.Transcript = append(g.result.Transcript, fmt.Sprintf("undecryptable %s %s", MemberName(to), w.text))
	case string(plaintext) != w.text:
		return fmt.Errorf("%w: %s got %q, want %q", ErrWrongText, MemberName(to), plaintext, w.text)
	}
	return nil
}

func (g *group) release(to int) error {
	held := g.held[to]
	g.held[to] = nil
	for _, w := range held {
		if err := g.deliver(to, w); err != nil {
			return err
		}
	}
	return nil
}
