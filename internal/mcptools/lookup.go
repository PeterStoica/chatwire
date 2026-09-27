package mcptools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

const maxLookUps = 10

type LookUpInput struct {
	Numbers []string `json:"numbers" jsonschema:"mobile numbers with country code, at most 10"`
}

type NumberInfo struct {
	Number     string `json:"number"`
	OnWhatsApp bool   `json:"on_whatsapp"`
	Name       string `json:"name,omitempty"`
	About      string `json:"about,omitempty"`
}

type LookUpReport struct {
	State   string       `json:"state"`
	Numbers []NumberInfo `json:"numbers"`
	Detail  string       `json:"detail"`
}

const lookUpDescription = "Check whether mobile numbers are on WhatsApp, and show the name the user saved and the person's about text when WhatsApp shares it. " +
	"At most 10 numbers at a time: looking up many strangers makes WhatsApp suspect a spammer."

func lookUp(s Sender) mcp.ToolHandlerFor[LookUpInput, LookUpReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in LookUpInput) (*mcp.CallToolResult, LookUpReport, error) {
		refuse := func(state, detail string) (*mcp.CallToolResult, LookUpReport, error) {
			return reply(LookUpReport{State: state, Numbers: []NumberInfo{}, Detail: detail})
		}
		if _, linked := s.Self(); !linked {
			return refuse(stateNotLinked, notLinked)
		}
		switch {
		case len(in.Numbers) == 0:
			return refuse("no_numbers", "Give at least one mobile number with country code.")
		case len(in.Numbers) > maxLookUps:
			return refuse("too_many", fmt.Sprintf("At most %d numbers at a time; looking up many strangers makes WhatsApp suspect a spammer.", maxLookUps))
		}
		numbers := make([]string, 0, len(in.Numbers))
		for _, raw := range in.Numbers {
			phone, err := pairing.ParsePhone(raw)
			if err != nil {
				return refuse("invalid_number", fmt.Sprintf("%q is not a mobile number with country code.", raw))
			}
			numbers = append(numbers, string(phone))
		}
		ctx, cancel := context.WithTimeout(ctx, sendTimeout)
		defer cancel()
		found, err := s.LookUp(ctx, numbers)
		if err != nil {
			return refuse(stateFailed, fmt.Sprintf("Could not look the numbers up: %v", err))
		}
		dir, _ := loadDirectory(ctx, s)
		report := LookUpReport{State: "ok", Numbers: make([]NumberInfo, 0, len(found))}
		var on []string
		for _, f := range found {
			info := NumberInfo{Number: "+" + f.Number, OnWhatsApp: f.OnWhatsApp, About: f.About}
			if f.OnWhatsApp {
				info.Name = dir.name(node.JID{User: f.Number, Server: node.ServerUser})
				on = append(on, info.Number)
			}
			report.Numbers = append(report.Numbers, info)
		}
		report.Detail = fmt.Sprintf("%d of %d on WhatsApp", len(on), len(found))
		if len(on) > 0 {
			report.Detail += ": " + strings.Join(on, ", ")
		}
		report.Detail += "."
		return reply(report)
	}
}
