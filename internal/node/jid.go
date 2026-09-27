package node

import (
	"fmt"
	"strconv"
	"strings"
)

type Server string

const (
	ServerUser       Server = "s.whatsapp.net"
	ServerLID        Server = "lid"
	ServerHosted     Server = "hosted"
	ServerHostedLID  Server = "hosted.lid"
	ServerGroup      Server = "g.us"
	ServerBroadcast  Server = "broadcast"
	ServerNewsletter Server = "newsletter"
	ServerBot        Server = "bot"

	legacyUser   = "c.us"
	hostedDevice = 99
)

const (
	domainUser      byte = 0
	domainLID       byte = 1
	domainHosted    byte = 128
	domainHostedLID byte = 129
)

type JID struct {
	User   string
	Device uint8
	Server Server
}

func (j JID) String() string {
	switch {
	case j.Server == "":
		return ""
	case j.Device > 0:
		return j.User + ":" + strconv.Itoa(int(j.Device)) + "@" + string(j.Server)
	case j.User != "":
		return j.User + "@" + string(j.Server)
	default:
		return string(j.Server)
	}
}

func StatusBroadcast() JID {
	return JID{User: "status", Server: ServerBroadcast}
}

func (j JID) IsStatus() bool {
	return j.Server == ServerBroadcast && strings.EqualFold(j.User, "status")
}

func (j JID) WithoutDevice() JID {
	return JID{User: j.User, Server: j.Server}
}

func (j JID) encodable() error {
	switch {
	case j.Server == "":
		return fmt.Errorf("%w: jid %q has no server", ErrUnencodable, j.User)
	case j.Device > 0 && !j.addressedByDevice():
		return fmt.Errorf("%w: device on %s", ErrUnencodable, j.Server)
	default:
		return nil
	}
}

func (j JID) addressedByDevice() bool {
	switch j.Server {
	case ServerHosted, ServerHostedLID:
		return true
	case ServerUser, ServerLID:
		return j.Device > 0
	default:
		return false
	}
}

func (j JID) domain() byte {
	switch j.Server {
	case ServerLID:
		return domainLID
	case ServerHosted:
		return domainHosted
	case ServerHostedLID:
		return domainHostedLID
	default:
		return domainUser
	}
}

func jidFromDomain(user string, domain, device byte) (JID, error) {
	j := JID{User: user, Device: device}
	switch {
	case domain == domainUser:
		j.Server = ServerUser
	case domain == domainLID:
		j.Server = ServerLID
	case domain == domainHostedLID:
		j.Server = ServerHostedLID
	case domain&domainLID == 0 && domain&domainHosted != 0:
		j.Server = ServerHosted
	default:
		return JID{}, fmt.Errorf("%w: %d", ErrJIDDomain, domain)
	}
	return j, nil
}

func ParseJID(s string) (JID, error) {
	user, host, found := strings.CutLast(s, "@")
	if !found {
		return JID{}, fmt.Errorf("%w: %q has no server", ErrJID, s)
	}
	server := Server(host)
	var (
		j  JID
		ok bool
	)
	switch server {
	case ServerUser, legacyUser:
		j, ok = device(user, phone)
		if user == "0" {
			j, ok = JID{User: user}, true
		}
		j.Server = ServerUser
	case ServerLID:
		j, ok = device(user, lid)
		j.Server = ServerLID
	case ServerHosted, ServerHostedLID:
		valid := phone
		if server == ServerHostedLID {
			valid = lid
		}
		base, found := strings.CutSuffix(user, ":99")
		j, ok = JID{User: base, Device: hostedDevice, Server: server}, found && valid(base)
	case ServerGroup:
		creator, created, legacy := strings.Cut(user, "-")
		j, ok = JID{User: user, Server: server}, legacy && phone(creator) && number(created, 10, 10) || number(user, 1, 20)
	case ServerBroadcast:
		j, ok = JID{User: user, Server: server}, number(user, 1, 20) || strings.EqualFold(user, "status") || strings.EqualFold(user, "location") || strings.EqualFold(user, "chat")
	case ServerNewsletter:
		j, ok = JID{User: user, Server: server}, number(user, 1, 20)
	case ServerBot:
		base, _ := strings.CutSuffix(user, ":0")
		j, ok = JID{User: base, Server: server}, number(base, 1, 20)
	}
	if !ok {
		return JID{}, fmt.Errorf("%w: %q", ErrJID, s)
	}
	return j, nil
}

func device(user string, valid func(string) bool) (JID, bool) {
	base, suffix, hasDevice := strings.Cut(user, ":")
	base = strings.TrimSuffix(base, ".0")
	if !valid(base) {
		return JID{}, false
	}
	if !hasDevice {
		return JID{User: base}, true
	}
	if len(suffix) < 1 || len(suffix) > 2 || !digits(suffix) {
		return JID{}, false
	}
	device := suffix[0] - '0'
	if len(suffix) == 2 {
		device = device*10 + suffix[1] - '0'
	}
	return JID{User: base, Device: device}, true
}

func phone(s string) bool { return number(s, 5, 20) && !strings.HasPrefix(s, "10") }

func lid(s string) bool { return number(s, 1, 15) }

func number(s string, shortest, longest int) bool {
	return len(s) >= shortest && len(s) <= longest && s[0] != '0' && digits(s)
}

func digits(s string) bool {
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
