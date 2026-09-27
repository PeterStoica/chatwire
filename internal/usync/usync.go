package usync

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/PeterStoica/chatwire/internal/node"
)

type Context string

const (
	ContextMessage     Context = "message"
	ContextInteractive Context = "interactive"
	ContextBackground  Context = "background"
	devicesVersion             = "2"
	tagDevices                 = "devices"
)

var (
	ErrReply   = errors.New("usync: unexpected reply")
	ErrDevices = errors.New("usync: WhatsApp could not list the devices")
)

type ServerError struct {
	Code int
	Text string
}

func (f ServerError) Error() string {
	return fmt.Sprintf("%d %s", f.Code, f.Text)
}

type Device struct {
	JID      node.JID
	KeyIndex uint32
}

type User struct {
	JID          node.JID
	Devices      []Device
	KeyIndexList []byte
	Err          error
}

func DevicesRequest(sid string, context Context, users []node.JID) node.Node {
	list := make([]node.Node, len(users))
	for i, user := range users {
		list[i] = node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: node.Address(user.WithoutDevice())}}}
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: "xmlns", Value: node.Text("usync")},
			{Key: "type", Value: node.Text("get")}, {Key: "id", Value: node.Value{}},
		},
		Children: []node.Node{{
			Tag: "usync",
			Attrs: []node.Attr{
				{Key: "sid", Value: node.Text(sid)}, {Key: "index", Value: node.Text("0")}, {Key: "last", Value: node.Text("true")},
				{Key: "mode", Value: node.Text("query")}, {Key: "context", Value: node.Text(string(context))},
			},
			Children: []node.Node{
				{Tag: "query", Children: []node.Node{{Tag: tagDevices, Attrs: []node.Attr{{Key: "version", Value: node.Text(devicesVersion)}}}}},
				{Tag: "list", Children: list},
			},
		}},
	}
}

func ParseDevices(reply node.Node) ([]User, error) {
	usync, ok := reply.Child("usync")
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "result" || !ok {
		return nil, fmt.Errorf("%w: %s", ErrReply, reply)
	}
	result, _ := usync.Child("result")
	protocol, _ := result.Child(tagDevices)
	if failure, failed := protocol.Child("error"); failed {
		return nil, fmt.Errorf("%w: %w", ErrDevices, failureOf(failure))
	}
	list, _ := usync.Child("list")
	users := make([]User, 0, len(list.Children))
	for _, entry := range list.Children {
		jid, ok := entry.Attr("jid").JID()
		if entry.Tag != "user" || !ok {
			continue
		}
		user, err := parseUser(jid, entry)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func parseUser(jid node.JID, entry node.Node) (User, error) {
	user := User{JID: jid}
	devices, ok := entry.Child(tagDevices)
	if !ok {
		return user, nil
	}
	if failure, failed := devices.Child("error"); failed {
		user.Err = fmt.Errorf("%w: %s: %w", ErrDevices, jid, failureOf(failure))
		return user, nil
	}
	if keyIndex, ok := devices.Child("key-index-list"); ok {
		user.KeyIndexList = keyIndex.Bytes
	}
	list, _ := devices.Child("device-list")
	for _, d := range list.Children {
		if d.Tag != "device" {
			continue
		}
		id, err := number(d, "id")
		if err != nil || id > math.MaxUint8 {
			return User{}, fmt.Errorf("%w: device id in %s", ErrReply, d)
		}
		device := Device{JID: node.JID{User: jid.User, Device: uint8(id), Server: jid.Server}}
		if hosted, _ := d.Attr("is_hosted").Text(); hosted == "true" {
			device.JID.Server = hostedServer(jid.Server)
		}
		if !d.Attr("key-index").IsZero() {
			index, err := number(d, "key-index")
			if err != nil || index > math.MaxUint32 {
				return User{}, fmt.Errorf("%w: key index in %s", ErrReply, d)
			}
			device.KeyIndex = uint32(index)
		}
		user.Devices = append(user.Devices, device)
	}
	return user, nil
}

func hostedServer(server node.Server) node.Server {
	if server == node.ServerLID {
		return node.ServerHostedLID
	}
	return node.ServerHosted
}

func number(n node.Node, key string) (uint64, error) {
	text, _ := n.Attr(key).Text()
	return strconv.ParseUint(text, 10, 64)
}

func failureOf(failure node.Node) ServerError {
	code, _ := failure.Attr("code").Text()
	text, _ := failure.Attr("text").Text()
	number, _ := strconv.Atoi(code)
	return ServerError{Code: number, Text: text}
}
