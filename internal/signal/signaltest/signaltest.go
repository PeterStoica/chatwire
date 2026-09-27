package signaltest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand/v2"
	"strings"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/signal"
)

const (
	Rounds               = 40
	SignedPreKeyID       = 7
	FirstOneTimePreKeyID = 100
	OneTimePreKeys       = 4
	crossHold            = 2
	registrationMask     = 0x3fff
)

var ErrWrongText = errors.New("signaltest: decrypted the wrong plaintext")

type Profile struct {
	Name         string
	Identity     curve.KeyPair
	Registration uint32
	Signed       curve.KeyPair
	Signature    curve.Signature
	OneTime      map[uint32]curve.KeyPair
}

func Profiles(seed uint64) ([2]Profile, error) {
	var key [32]byte
	binary.LittleEndian.PutUint64(key[:], seed)
	random := mathrand.NewChaCha8(key)
	alice, err := newProfile(random, "alice")
	if err != nil {
		return [2]Profile{}, err
	}
	bob, err := newProfile(random, "bob")
	return [2]Profile{alice, bob}, err
}

func newProfile(random *mathrand.ChaCha8, name string) (Profile, error) {
	var registration [4]byte
	if _, err := random.Read(registration[:]); err != nil {
		return Profile{}, err
	}
	pairs := make([]curve.KeyPair, 2+OneTimePreKeys)
	for i := range pairs {
		pair, err := curve.NewKeyPair(random)
		if err != nil {
			return Profile{}, err
		}
		pairs[i] = pair
	}
	signature, err := pairs[0].SignKey(random, pairs[1].Public())
	if err != nil {
		return Profile{}, err
	}
	p := Profile{
		Name: name, Identity: pairs[0], Registration: binary.LittleEndian.Uint32(registration[:])&registrationMask + 1,
		Signed: pairs[1], Signature: signature, OneTime: map[uint32]curve.KeyPair{},
	}
	for i, pair := range pairs[2:] {
		p.OneTime[FirstOneTimePreKeyID+uint32(i)] = pair
	}
	return p, nil
}

func (p Profile) Bundle(oneTime *uint32) signal.Bundle {
	b := signal.Bundle{
		RegistrationID: p.Registration, Identity: p.Identity.Public(),
		SignedPreKeyID: SignedPreKeyID, SignedPreKey: p.Signed.Public(), SignedPreKeySignature: p.Signature,
	}
	if oneTime != nil {
		b.PreKey = &signal.OneTimePreKey{ID: *oneTime, Key: p.OneTime[*oneTime].Public()}
	}
	return b
}

type Wire struct {
	PreKey bool
	Bytes  []byte
	Text   string
}

type Party interface {
	Initiate(peer signal.Bundle) error
	Send(text string) (Wire, error)
	Receive(w Wire) ([]byte, error)
}

type Factory func(self, peer Profile) Party

type ours struct {
	local   signal.Local
	profile Profile
	used    map[uint32]bool
	session *signal.Session
}

func NewOurs(self, _ Profile) Party {
	return &ours{local: signal.Local{Identity: self.Identity, RegistrationID: self.Registration}, profile: self, used: map[uint32]bool{}}
}

func (o *ours) PreKey(id uint32) (curve.KeyPair, bool) {
	pair, ok := o.profile.OneTime[id]
	return pair, ok && !o.used[id]
}

func (o *ours) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return o.profile.Signed, id == SignedPreKeyID
}

func (o *ours) Initiate(peer signal.Bundle) error {
	s, err := signal.Initiate(rand.Reader, o.local, o.session, peer)
	if err == nil {
		o.session = s
	}
	return err
}

func (o *ours) Send(text string) (Wire, error) {
	ct, err := o.session.Encrypt([]byte(text))
	return Wire{PreKey: ct.Type == signal.TypePreKeyMessage, Bytes: ct.Bytes, Text: text}, err
}

func (o *ours) Receive(w Wire) ([]byte, error) {
	if !w.PreKey {
		return o.session.Decrypt(rand.Reader, w.Bytes)
	}
	s, plaintext, used, err := signal.Respond(rand.Reader, o.local, o, o.session, w.Bytes)
	if err != nil {
		return nil, err
	}
	o.session = s
	if used != nil {
		o.used[*used] = true
	}
	return plaintext, nil
}

type Result struct {
	Transcript []string
	Stats      map[string]int
}

func (r Result) Digest() string {
	sum := sha256.Sum256([]byte(strings.Join(r.Transcript, "\n")))
	return hex.EncodeToString(sum[:])
}

func (r Result) Undecryptable() []string {
	var out []string
	for _, event := range r.Transcript {
		if text, ok := strings.CutPrefix(event, "undecryptable "); ok {
			out = append(out, text)
		}
	}
	return out
}

type held struct {
	Wire
	since       int
	reinitiated bool
}

type conversation struct {
	schedule     *mathrand.Rand
	crossSession bool
	profiles     [2]Profile
	parties      [2]Party
	handed       [2]uint32
	held         [2][]held
	result       Result
}

func Converse(seed uint64, crossSession bool, profiles [2]Profile, factories [2]Factory) (Result, error) {
	c := &conversation{
		schedule: mathrand.New(mathrand.NewPCG(seed, 0x5157)), crossSession: crossSession, profiles: profiles,
		parties: [2]Party{factories[0](profiles[0], profiles[1]), factories[1](profiles[1], profiles[0])},
		result:  Result{Stats: map[string]int{}},
	}
	if err := c.initiate(0); err != nil {
		return c.result, err
	}
	for round := range Rounds {
		if err := c.round(round); err != nil {
			return c.result, fmt.Errorf("round %d: %w", round, err)
		}
	}
	for side := range c.held {
		if err := c.release(side, nil); err != nil {
			return c.result, err
		}
	}
	return c.result, nil
}

func (r Result) count(stat string, n int) {
	r.Stats[stat] += n
}

func (c *conversation) nextBundle(side int) signal.Bundle {
	if c.handed[side] == OneTimePreKeys || c.schedule.IntN(4) == 0 {
		c.result.count("bundles without one-time prekey", 1)
		return c.profiles[side].Bundle(nil)
	}
	id := FirstOneTimePreKeyID + c.handed[side]
	c.handed[side]++
	return c.profiles[side].Bundle(&id)
}

func (c *conversation) send(from int, text string) (Wire, error) {
	w, err := c.parties[from].Send(text)
	if err != nil {
		return w, fmt.Errorf("%s sends %s: %w", c.profiles[from].Name, text, err)
	}
	c.result.Transcript = append(c.result.Transcript, fmt.Sprintf("send %x", w.Bytes))
	if w.PreKey {
		c.result.count("prekey messages", 1)
	}
	return w, nil
}

func (c *conversation) deliver(to int, w Wire) error {
	c.result.count("deliveries", 1)
	plaintext, err := c.parties[to].Receive(w)
	switch {
	case err != nil:
		c.result.count("undecryptable", 1)
		c.result.Transcript = append(c.result.Transcript, "undecryptable "+w.Text)
	case string(plaintext) != w.Text:
		return fmt.Errorf("%w: %s got %q, want %q", ErrWrongText, c.profiles[to].Name, plaintext, w.Text)
	}
	return nil
}

func (c *conversation) initiate(from int) error {
	if err := c.parties[from].Initiate(c.nextBundle(1 - from)); err != nil {
		return fmt.Errorf("%s initiates: %w", c.profiles[from].Name, err)
	}
	w, err := c.send(from, fmt.Sprintf("session from %d", from))
	if err != nil {
		return err
	}
	return c.deliver(1-from, w)
}

func (c *conversation) reinitiate(from int) error {
	c.result.count("re-initiations", 1)
	if !c.crossSession {
		for side := range c.held {
			if err := c.release(side, nil); err != nil {
				return err
			}
		}
	}
	for side := range c.held {
		for i := range c.held[side] {
			c.held[side][i].reinitiated = true
		}
	}
	return c.initiate(from)
}

func (c *conversation) round(round int) error {
	from := c.schedule.IntN(2)
	to := 1 - from
	if c.schedule.IntN(12) == 0 {
		return c.reinitiate(from)
	}
	batch := make([]Wire, 1+c.schedule.IntN(5))
	for i := range batch {
		w, err := c.send(from, fmt.Sprintf("r%d-%d-%d", round, from, i))
		if err != nil {
			return err
		}
		batch[i] = w
	}
	c.schedule.Shuffle(len(batch), func(i, j int) { batch[i], batch[j] = batch[j], batch[i] })
	if len(batch) > 1 && c.schedule.IntN(3) == 0 {
		c.result.count("held back", 1)
		c.held[to] = append(c.held[to], held{Wire: batch[len(batch)-1], since: round})
		batch = batch[:len(batch)-1]
	}
	for _, w := range batch {
		if err := c.deliver(to, w); err != nil {
			return err
		}
	}
	switch {
	case c.schedule.IntN(5) == 0:
		return c.release(to, nil)
	case c.crossSession:
		return c.release(to, func(h held) bool { return round-h.since >= crossHold })
	}
	return nil
}

func (c *conversation) release(side int, due func(held) bool) error {
	var keep []held
	for _, h := range c.held[side] {
		if due != nil && !due(h) {
			keep = append(keep, h)
			continue
		}
		if h.reinitiated {
			c.result.count("delivered across a re-initiation", 1)
		}
		if err := c.deliver(side, h.Wire); err != nil {
			return err
		}
	}
	c.held[side] = keep
	return nil
}
