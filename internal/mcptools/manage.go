package mcptools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
)

type GroupInput struct {
	Group  string   `json:"group" jsonschema:"the group, by the name the user calls it"`
	Action string   `json:"action" jsonschema:"one of: rename, describe, add, remove, make_admin, remove_admin, leave"`
	Text   string   `json:"text,omitempty" jsonschema:"for rename the new name; for describe the new description (empty removes it)"`
	People []string `json:"people,omitempty" jsonschema:"for add, remove, make_admin and remove_admin: contact names or mobile numbers with country code"`
}

type MemberProblem struct {
	Person string `json:"person"`
	Reason string `json:"reason"`
}

type GroupReport struct {
	State   string          `json:"state"`
	Group   string          `json:"group,omitempty"`
	Changed []string        `json:"changed,omitempty"`
	Failed  []MemberProblem `json:"failed,omitempty"`
	Detail  string          `json:"detail"`
}

const manageDescription = "Change a WhatsApp group the user is in: rename it, set its description, add or remove people, make someone an admin or not, or leave it. " +
	"Only when the user asks for that change. Most changes need the user to be a group admin."

func memberChange(action string) (groups.Change, bool) {
	switch action {
	case "add":
		return groups.Add, true
	case "remove":
		return groups.Remove, true
	case "make_admin":
		return groups.Promote, true
	case "remove_admin":
		return groups.Demote, true
	}
	return "", false
}

func manageGroup(s Sender) mcp.ToolHandlerFor[GroupInput, GroupReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GroupInput) (*mcp.CallToolResult, GroupReport, error) {
		if _, linked := s.Self(); !linked {
			return reply(GroupReport{State: stateNotLinked, Detail: notLinked})
		}
		ctx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		dir, err := loadDirectory(ctx, s)
		if err != nil {
			return reply(GroupReport{State: stateFailed, Detail: fmt.Sprintf("Could not look up contacts: %v", err)})
		}
		group, refused := resolve(ctx, s, &dir, in.Group, "group")
		switch {
		case refused != nil:
			return reply(GroupReport{State: refused.state, Detail: refused.detail})
		case group.Server != node.ServerGroup:
			return reply(GroupReport{State: "not_a_group", Detail: fmt.Sprintf("%s is not a group.", dir.label(group))})
		}
		name := dir.label(group)
		action := strings.ToLower(strings.TrimSpace(in.Action))
		done := func(detail string) (*mcp.CallToolResult, GroupReport, error) {
			return reply(GroupReport{State: "done", Group: name, Detail: detail})
		}
		switch action {
		case "rename":
			err = s.RenameGroup(ctx, group, in.Text)
			if err == nil {
				return done(fmt.Sprintf("Renamed %s to %q.", name, strings.TrimSpace(in.Text)))
			}
		case "describe":
			err = s.DescribeGroup(ctx, group, in.Text)
			if err == nil && strings.TrimSpace(in.Text) == "" {
				return done(fmt.Sprintf("Removed the description of %s.", name))
			} else if err == nil {
				return done(fmt.Sprintf("Set the description of %s.", name))
			}
		case "leave":
			err = s.LeaveGroup(ctx, group)
			if err == nil {
				return done(fmt.Sprintf("Left %s.", name))
			}
		default:
			change, ok := memberChange(action)
			if !ok {
				return reply(GroupReport{State: "unknown_action", Group: name, Detail: "action must be one of: rename, describe, add, remove, make_admin, remove_admin, leave."})
			}
			return reply(changeMembers(ctx, s, &dir, group, name, change, in.People))
		}
		return reply(groupRefusal(name, err))
	}
}

func changeMembers(ctx context.Context, s Sender, dir *directory, group node.JID, name string, change groups.Change, asked []string) GroupReport {
	people := make([]node.JID, 0, len(asked))
	for _, who := range asked {
		person, refused := resolve(ctx, s, dir, who, "person")
		switch {
		case refused != nil:
			return GroupReport{State: refused.state, Group: name, Detail: refused.detail}
		case person.Server == node.ServerGroup || person.Server == node.ServerBroadcast:
			return GroupReport{State: "not_a_person", Group: name, Detail: fmt.Sprintf("%s is a group, not a person.", dir.label(person))}
		}
		people = append(people, person)
	}
	outcomes, err := s.ChangeMembers(ctx, group, change, people)
	if err != nil {
		return groupRefusal(name, err)
	}
	report := GroupReport{State: "done", Group: name}
	for _, o := range outcomes {
		if o.Code == 0 {
			report.Changed = append(report.Changed, dir.label(o.Person))
			continue
		}
		report.Failed = append(report.Failed, MemberProblem{Person: dir.label(o.Person), Reason: memberReason(change, o.Code)})
	}
	verb := map[groups.Change]string{groups.Add: "Added", groups.Remove: "Removed", groups.Promote: "Made admin", groups.Demote: "No longer admin"}[change]
	switch {
	case len(report.Failed) == 0:
		report.Detail = fmt.Sprintf("%s in %s: %s.", verb, name, strings.Join(report.Changed, ", "))
	case len(report.Changed) == 0:
		report.State, report.Detail = "not_changed", "Nobody was changed; see failed for why."
	default:
		report.State, report.Detail = "partly_done", fmt.Sprintf("%s in %s: %s. Some were not; see failed for why.", verb, name, strings.Join(report.Changed, ", "))
	}
	return report
}

func memberReason(change groups.Change, code int) string {
	switch {
	case code == 403 && change == groups.Add:
		return "their privacy settings only let people add them by invite; ask the user to send them the group's invite link from the phone"
	case code == 408 && change == groups.Add:
		return "they left the group recently, so WhatsApp does not let them be added back yet"
	case code == 409 && change == groups.Add:
		return "already in the group"
	case code == 404:
		return "not in the group"
	}
	return fmt.Sprintf("WhatsApp refused it (error %d)", code)
}

func groupRefusal(name string, err error) GroupReport {
	var refused groups.Refused
	switch {
	case errors.Is(err, messenger.ErrSubject), errors.Is(err, messenger.ErrDescription), errors.Is(err, messenger.ErrNobody):
		return GroupReport{State: "invalid", Group: name, Detail: strings.TrimPrefix(err.Error(), "messenger: ") + "."}
	case errors.Is(err, messenger.ErrNotMember):
		return GroupReport{State: "not_a_member", Group: name, Detail: "The user is not in this group any more."}
	case errors.As(err, &refused) && (refused.Code == 401 || refused.Code == 403):
		return GroupReport{State: "not_admin", Group: name, Detail: "Only group admins can do that, and the user is not an admin of " + name + "."}
	case errors.As(err, &refused) && refused.Code == 409:
		return GroupReport{State: "try_again", Group: name, Detail: "Someone changed the group at the same time; try again."}
	case errors.As(err, &refused):
		return GroupReport{State: "rejected", Group: name, Detail: fmt.Sprintf("WhatsApp refused the change (error %d).", refused.Code)}
	}
	return GroupReport{State: stateFailed, Group: name, Detail: fmt.Sprintf("The group was not changed: %v", err)}
}
