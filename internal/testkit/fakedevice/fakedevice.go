package fakedevice

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/usync"
	"github.com/PeterStoica/chatwire/internal/wire"
)

var ErrUndecryptable = errors.New("fakedevice: cannot decrypt")

type Device struct {
	JID      node.JID
	Identity device.Identity
	random   io.Reader
	next     uint32
	oneTime  map[uint32]curve.KeyPair
	sessions map[node.JID]*signal.Session
	groups   map[[2]node.JID]*signal.SenderKeys
}

func New(random io.Reader, jid node.JID) (*Device, error) {
	identity, err := device.New(random)
	if err != nil {
		return nil, err
	}
	return &Device{JID: jid, Identity: identity, random: random, next: 1, oneTime: map[uint32]curve.KeyPair{}, sessions: map[node.JID]*signal.Session{}, groups: map[[2]node.JID]*signal.SenderKeys{}}, nil
}

func (d *Device) Upload(keys *fakekeys.Server, count int) error {
	generated, next, err := prekeys.Generate(d.random, d.next, count)
	if err != nil {
		return err
	}
	d.next = next
	for _, k := range generated {
		d.oneTime[k.ID] = k.Key
	}
	return prekeys.Result(keys.Handle(d.JID, prekeys.Upload(d.Identity.Registration(device.Props()), generated)))
}

func (d *Device) AskAgain(stanza node.Node, count int) (node.Node, error) {
	in, err := message.ParseIncoming(d.mine, stanza)
	if err != nil {
		return node.Node{}, err
	}
	fresh, next, err := prekeys.Generate(d.random, d.next, 1)
	if err != nil {
		return node.Node{}, err
	}
	d.next = next
	d.oneTime[fresh[0].ID] = fresh[0].Key
	return message.RetryReceipt(d.mine, in, message.Retry{
		Count: count, Registration: d.Identity.Registration(device.Props()), PreKey: fresh[0], DeviceIdentity: []byte("simulated device identity"),
	})
}

func (d *Device) PreKey(id uint32) (curve.KeyPair, bool) {
	key, ok := d.oneTime[id]
	return key, ok
}

func (d *Device) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return d.Identity.SignedPreKey(), id == d.Identity.Registration(device.Props()).SignedPreKey.ID
}

func (d *Device) mine(j node.JID) bool {
	return j.User == d.JID.User && j.Server == d.JID.Server
}

func (d *Device) Receive(stanza node.Node) (message.Incoming, *wire.Message, error) {
	in, err := message.ParseIncoming(d.mine, stanza)
	if err != nil {
		return in, nil, err
	}
	var content *wire.Message
	for _, enc := range in.Encs {
		if enc.Type == "skmsg" {
			continue
		}
		if content, err = d.pairwiseWithKeys(in, enc); err != nil {
			return in, nil, err
		}
	}
	for _, enc := range in.Encs {
		if enc.Type == "skmsg" {
			if content, err = d.group(in, enc); err != nil {
				return in, nil, err
			}
		}
	}
	if content == nil {
		return in, nil, fmt.Errorf("%w: nothing to decrypt", ErrUndecryptable)
	}
	return in, content, nil
}

func (d *Device) pairwiseWithKeys(in message.Incoming, enc message.Enc) (*wire.Message, error) {
	m, err := d.pairwise(in, enc)
	if err != nil {
		return nil, err
	}
	if distribution := m.GetSenderKeyDistributionMessage(); distribution != nil {
		name := [2]node.JID{in.Chat, in.Author}
		if d.groups[name] == nil {
			d.groups[name] = &signal.SenderKeys{}
		}
		if err := d.groups[name].Process(distribution.GetAxolotlSenderKeyDistributionMessage()); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (d *Device) group(in message.Incoming, enc message.Enc) (*wire.Message, error) {
	keys := d.groups[[2]node.JID{in.Chat, in.Author}]
	if keys == nil {
		return nil, fmt.Errorf("%w: no sender key from %s in %s", ErrUndecryptable, in.Author, in.Chat)
	}
	padded, err := keys.Decrypt(enc.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUndecryptable, err)
	}
	return message.Decode(padded)
}

func (d *Device) pairwise(in message.Incoming, enc message.Enc) (*wire.Message, error) {
	var padded []byte
	switch enc.Type {
	case "pkmsg":
		session, plaintext, used, err := signal.Respond(d.random, d.Identity.Signal(), d, d.sessions[in.Author], enc.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUndecryptable, err)
		}
		d.sessions[in.Author], padded = session, plaintext
		if used != nil {
			delete(d.oneTime, *used)
		}
	case "msg":
		plaintext, err := d.sessions[in.Author].Decrypt(d.random, enc.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUndecryptable, err)
		}
		padded = plaintext
	default:
		return nil, fmt.Errorf("%w: %s", ErrUndecryptable, enc.Type)
	}
	return message.Decode(padded)
}

func (d *Device) Send(keys *fakekeys.Server, devices *fakeusync.Server, to node.JID, m *wire.Message) (node.Node, error) {
	self := d.JID.WithoutDevice()
	targets, err := d.targets(devices, to, self)
	if err != nil {
		return node.Node{}, err
	}
	if err := d.startSessions(keys, targets); err != nil {
		return node.Node{}, err
	}
	parts := make([]message.Part, 0, len(targets))
	for _, target := range targets {
		part, err := d.encrypt(target, to, self, m)
		if err != nil {
			return node.Node{}, err
		}
		parts = append(parts, part)
	}
	id, err := message.NewID(time.Now(), self, d.random)
	if err != nil {
		return node.Node{}, err
	}
	return message.Outgoing(id, to, m, parts, []byte("simulated device identity")), nil
}

func (d *Device) targets(devices *fakeusync.Server, to, self node.JID) ([]node.JID, error) {
	users, err := usync.ParseDevices(devices.Handle(usync.DevicesRequest("9.9-9", usync.ContextMessage, []node.JID{to, self})))
	if err != nil {
		return nil, err
	}
	var targets []node.JID
	for _, u := range users {
		for _, target := range u.Devices {
			if target.JID != d.JID {
				targets = append(targets, target.JID)
			}
		}
	}
	return targets, nil
}

func (d *Device) startSessions(keys *fakekeys.Server, targets []node.JID) error {
	var missing []node.JID
	for _, target := range targets {
		if d.sessions[target] == nil {
			missing = append(missing, target)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	bundles, _, err := prekeys.ParseBundles(keys.Handle(d.JID, prekeys.FetchRequest(missing)))
	if err != nil {
		return err
	}
	for _, b := range bundles {
		if d.sessions[b.Device], err = signal.Initiate(d.random, d.Identity.Signal(), nil, b.Keys); err != nil {
			return err
		}
	}
	return nil
}

func (d *Device) encrypt(target, to, self node.JID, m *wire.Message) (message.Part, error) {
	payload := m
	if target.User == self.User && target.Server == self.Server {
		payload = message.SentByUs(to, m)
	}
	padded, err := message.Encode(d.random, payload)
	if err != nil {
		return message.Part{}, err
	}
	ciphertext, err := d.sessions[target].Encrypt(padded)
	if err != nil {
		return message.Part{}, err
	}
	return message.Part{Device: target, Ciphertext: ciphertext}, nil
}

func (d *Device) EncryptFor(keys *fakekeys.Server, target node.JID, m *wire.Message) (message.Part, error) {
	if err := d.startSessions(keys, []node.JID{target}); err != nil {
		return message.Part{}, err
	}
	return d.encrypt(target, target.WithoutDevice(), d.JID.WithoutDevice(), m)
}

func (d *Device) SendPeer(keys *fakekeys.Server, target node.JID, m *wire.Message) (node.Node, error) {
	padded, err := message.Encode(d.random, m)
	if err != nil {
		return node.Node{}, err
	}
	return d.sendPadded(keys, target, m, padded)
}

func (d *Device) SendUnreadable(keys *fakekeys.Server, target node.JID) (node.Node, error) {
	return d.sendPadded(keys, target, &wire.Message{Conversation: new("")}, []byte{0x0a, 0xff, 0x01, 0x01})
}

func (d *Device) sendPadded(keys *fakekeys.Server, target node.JID, m *wire.Message, padded []byte) (node.Node, error) {
	if err := d.startSessions(keys, []node.JID{target}); err != nil {
		return node.Node{}, err
	}
	ciphertext, err := d.sessions[target].Encrypt(padded)
	if err != nil {
		return node.Node{}, err
	}
	self := d.JID.WithoutDevice()
	id, err := message.NewID(time.Now(), self, d.random)
	if err != nil {
		return node.Node{}, err
	}
	return message.Outgoing(id, target, m, []message.Part{{Device: target, Ciphertext: ciphertext}}, []byte("simulated device identity")), nil
}
