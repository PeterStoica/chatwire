package client

import "github.com/PeterStoica/chatwire/internal/node"

type address node.JID

func (a address) String() string {
	return node.JID(a).String()
}

func (c *Client) addressLocked(j node.JID) address {
	if j.Server != node.ServerUser {
		return address(j)
	}
	if lid, ok := c.lids[j.User]; ok {
		return address{User: lid, Device: j.Device, Server: node.ServerLID}
	}
	return address(j)
}

func (c *Client) numberLocked(j node.JID) (node.JID, bool) {
	switch j.Server {
	case node.ServerUser:
		return j.WithoutDevice(), true
	case node.ServerLID:
		for pn, lid := range c.lids {
			if lid == j.User {
				return node.JID{User: pn, Server: node.ServerUser}, true
			}
		}
	}
	return node.JID{}, false
}

func (c *Client) alternateLocked(j node.JID) node.JID {
	switch j.Server {
	case node.ServerUser:
		if lid, ok := c.lids[j.User]; ok {
			return node.JID{User: lid, Server: node.ServerLID}
		}
	case node.ServerLID:
		if pn, ok := c.numberLocked(j); ok {
			return pn
		}
	}
	return node.JID{}
}

func (c *Client) Learn(pairs map[node.JID]node.JID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.learnLocked(pairs) {
		_ = c.keepLocked()
	}
}

func (c *Client) learnLocked(pairs map[node.JID]node.JID) bool {
	changed := false
	for lid, pn := range pairs {
		if lid.Server != node.ServerLID || pn.Server != node.ServerUser || lid.User == "" || pn.User == "" || c.lids[pn.User] == lid.User {
			continue
		}
		c.lids[pn.User] = lid.User
		changed = true
	}
	if changed {
		c.readdressLocked()
	}
	return changed
}

func (c *Client) readdressLocked() {
	for from, session := range c.sessions {
		if to := c.addressLocked(node.JID(from)); to != from {
			delete(c.sessions, from)
			c.sessions[to] = c.sessions[to].Merge(session)
		}
	}
	for name, keys := range c.groups {
		to := senderName{group: name.group, sender: c.addressLocked(node.JID(name.sender))}
		if to == name {
			continue
		}
		delete(c.groups, name)
		if c.groups[to] == nil {
			c.groups[to] = keys
		}
	}
	for _, holders := range c.holders {
		for holder := range holders {
			if to := c.addressLocked(node.JID(holder)); to != holder {
				delete(holders, holder)
				holders[to] = true
			}
		}
	}
}
