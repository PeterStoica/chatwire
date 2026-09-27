package messenger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
)

var (
	ErrSubject     = fmt.Errorf("messenger: a group name must be 1 to %d characters", groups.MaxSubject)
	ErrDescription = fmt.Errorf("messenger: a group description can be at most %d characters", groups.MaxDescription)
	ErrNobody      = errors.New("messenger: say who to change")
)

type MemberOutcome struct {
	Person node.JID
	Code   int
}

func (m *Messenger) groupRequest(ctx context.Context, group node.JID, request func(*client.Client, groups.Group) (node.Node, error)) (node.Node, error) {
	c, err := m.connected(ctx)
	if err != nil {
		return node.Node{}, err
	}
	g, err := m.group(ctx, c, group)
	if err != nil {
		return node.Node{}, err
	}
	stanza, err := request(c, g)
	if err != nil {
		return node.Node{}, err
	}
	reply, err := c.Query(ctx, stanza)
	m.groupChanged(group)
	if refused := groups.RefusedBy(reply); refused != nil {
		return reply, refused
	}
	return reply, err
}

func (m *Messenger) RenameGroup(ctx context.Context, group node.JID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > groups.MaxSubject {
		return ErrSubject
	}
	_, err := m.groupRequest(ctx, group, func(*client.Client, groups.Group) (node.Node, error) {
		return groups.SetSubject(group, name), nil
	})
	return err
}

func (m *Messenger) DescribeGroup(ctx context.Context, group node.JID, text string) error {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > groups.MaxDescription {
		return ErrDescription
	}
	_, err := m.groupRequest(ctx, group, func(_ *client.Client, g groups.Group) (node.Node, error) {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return node.Node{}, err
		}
		return groups.SetDescription(group, strings.ToUpper(hex.EncodeToString(raw)), g.DescriptionID, text), nil
	})
	return err
}

func (m *Messenger) LeaveGroup(ctx context.Context, group node.JID) error {
	_, err := m.groupRequest(ctx, group, func(*client.Client, groups.Group) (node.Node, error) {
		return groups.Leave(group), nil
	})
	return err
}

func (m *Messenger) ChangeMembers(ctx context.Context, group node.JID, change groups.Change, people []node.JID) ([]MemberOutcome, error) {
	if len(people) == 0 {
		return nil, ErrNobody
	}
	asked := map[node.JID]node.JID{}
	reply, err := m.groupRequest(ctx, group, func(_ *client.Client, g groups.Group) (node.Node, error) {
		members := make([]groups.Member, 0, len(people))
		for _, person := range people {
			member, err := m.memberIn(ctx, g, person)
			if err != nil {
				return node.Node{}, err
			}
			asked[member.JID.WithoutDevice()] = person
			members = append(members, member)
		}
		return groups.ChangeMembers(group, change, members), nil
	})
	if err != nil {
		return nil, err
	}
	outcomes, err := groups.ParseChange(reply, change)
	if err != nil {
		return nil, err
	}
	out := make([]MemberOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		person, ok := asked[o.JID.WithoutDevice()]
		if !ok {
			person = o.JID
		}
		out = append(out, MemberOutcome{Person: person, Code: o.Code})
	}
	return out, nil
}

func (m *Messenger) memberIn(ctx context.Context, g groups.Group, person node.JID) (groups.Member, error) {
	forms, err := m.store.Forms(ctx, person)
	if err != nil {
		return groups.Member{}, err
	}
	for _, p := range g.Participants {
		for _, f := range forms {
			if p.JID.WithoutDevice() == f || p.LID.WithoutDevice() == f || p.Phone.WithoutDevice() == f {
				return groups.Member{JID: p.JID.WithoutDevice(), Phone: p.Phone}, nil
			}
		}
	}
	member := groups.Member{JID: person.WithoutDevice()}
	if g.AddressingMode != "lid" {
		return member, nil
	}
	for _, f := range forms {
		if f.Server == node.ServerLID {
			return groups.Member{JID: f, Phone: person.WithoutDevice()}, nil
		}
	}
	return member, nil
}
