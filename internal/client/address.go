package client

import "github.com/PeterStoica/chatwire/internal/node"

func (c *Client) addressLocked(j node.JID) node.JID {
	if j.Server != node.ServerUser {
		return j
	}
	if lid, ok := c.lids[j.User]; ok {
		return node.JID{User: lid, Device: j.Device, Server: node.ServerLID}
	}
	return j
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
	for device, session := range c.sessions {
		if to := c.addressLocked(device); to != device {
			delete(c.sessions, device)
			c.sessions[to] = c.sessions[to].Merge(session)
		}
	}
	for name, keys := range c.groups {
		to := senderName{group: name.group, sender: c.addressLocked(name.sender)}
		if to == name {
			continue
		}
		delete(c.groups, name)
		if c.groups[to] == nil {
			c.groups[to] = keys
		}
	}
	for group, holders := range c.holders {
		for holder := range holders {
			if to := c.addressLocked(holder); to != holder {
				delete(holders, holder)
				holders[to] = true
			}
		}
		c.holders[group] = holders
	}
}
