package client

import (
	"context"

	"github.com/PeterStoica/chatwire/internal/node"
)

func (c *Client) encryptNotice(n node.Node) {
	if _, low := n.Child("count"); low {
		c.enqueue(func(ctx context.Context) { _ = c.uploadPreKeys(ctx) })
		return
	}
	if _, changed := n.Child("identity"); !changed {
		return
	}
	from, ok := n.Attr("from").JID()
	if !ok || from.Device != 0 || c.mine(from) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if lid, ok := n.Attr("lid").JID(); ok && lid.Server == node.ServerLID && from.Server == node.ServerUser {
		c.learnLocked(map[node.JID]node.JID{lid.WithoutDevice(): from.WithoutDevice()})
	}
	c.forgetLocked(from, nil)
	_ = c.keepLocked()
}

func (c *Client) devicesNotice(n node.Node) {
	from, ok := n.Attr("from").JID()
	if !ok {
		return
	}
	c.mu.Lock()
	c.forgetDevicesLocked(from)
	c.mu.Unlock()
	if c.mine(from) {
		return
	}
	removed := map[uint8]bool{}
	for _, change := range n.Children {
		if change.Tag != "remove" {
			continue
		}
		for _, d := range change.Children {
			if device, ok := d.Attr("jid").JID(); ok && d.Tag == "device" {
				removed[device.Device] = true
			}
		}
	}
	if len(removed) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forgetLocked(from, removed)
	_ = c.keepLocked()
}

func (c *Client) forgetLocked(user node.JID, only map[uint8]bool) {
	c.forgetDevicesLocked(user)
	whose := c.addressLocked(user.WithoutDevice())
	matches := func(a address) bool {
		return a.User == whose.User && a.Server == whose.Server && (only == nil || only[a.Device])
	}
	for a := range c.sessions {
		if matches(a) {
			delete(c.sessions, a)
			delete(c.recreated, a)
		}
	}
	for _, holders := range c.holders {
		for h := range holders {
			if matches(h) {
				delete(holders, h)
			}
		}
	}
}
