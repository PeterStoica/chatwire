package mcptools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rsc.io/qr"

	"github.com/PeterStoica/chatwire/internal/linker"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

type Linker interface {
	Start(ctx context.Context, phone string) (linker.Status, error)
	Status() linker.Status
	Await(ctx context.Context, done func(linker.Status) bool) (linker.Status, error)
}

type LinkInput struct {
	PhoneNumber string `json:"phone_number,omitempty" jsonschema:"the user's WhatsApp mobile number with country code, for example +40 721 234 567"`
}

type StatusInput struct {
	WaitSeconds int `json:"wait_seconds,omitempty" jsonschema:"wait up to this many seconds (at most 50) for a link in progress to finish before answering"`
}

type Report struct {
	State      string `json:"state"`
	Connection string `json:"connection,omitempty"`
	Code       string `json:"code,omitempty"`
	LinkedAs   string `json:"linked_as,omitempty"`
	Page       string `json:"page,omitempty"`
	Update     string `json:"update_available,omitempty"`
	Detail     string `json:"detail"`
}

func link(l Linker, opts Options) mcp.ToolHandlerFor[LinkInput, Report] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in LinkInput) (*mcp.CallToolResult, Report, error) {
		st, err := l.Start(ctx, in.PhoneNumber)
		if errors.Is(err, pairing.ErrPhone) {
			report := Report{State: "invalid_phone_number", Detail: "That does not look like a mobile number with its country code. Ask the user for it in international form, for example +40 721 234 567."}
			return nil, report, nil
		}
		if err != nil {
			return nil, Report{}, fmt.Errorf("mcptools: link: %w", err)
		}
		report := describe(st)
		if st.Phase == linker.ShowingQR {
			return qrResult(st.QR, withPage(ctx, opts, report))
		}
		return nil, report, nil
	}
}

func status(l Linker, s Sender, opts Options) mcp.ToolHandlerFor[StatusInput, Report] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in StatusInput) (*mcp.CallToolResult, Report, error) {
		st := l.Status()
		if st.Phase.InFlight() {
			wait := time.Duration(max(0, in.WaitSeconds)) * time.Second
			waitCtx, cancel := context.WithTimeout(ctx, min(wait, maxWait))
			defer cancel()
			start := st
			st, _ = l.Await(waitCtx, func(s linker.Status) bool { return !s.Phase.InFlight() || s.Code != start.Code })
		}
		report := describe(st)
		if st.Phase == linker.Linked {
			report.Connection, report.Detail = connection(s.Connection(), report.Detail)
		}
		if opts.Update != nil {
			if latest, newer := opts.Update(); newer {
				report.Update = latest
				report.Detail += fmt.Sprintf(" Chatwire %s is available; the user can install it by running: chatwire update", latest)
			}
		}
		return nil, report, nil
	}
}

func connection(c messenger.Connection, detail string) (string, string) {
	if c.StoreErr != nil {
		detail += fmt.Sprintf(" Saving messages failed (%v); check free disk space.", c.StoreErr)
	}
	if h := c.History; h.Started && h.Percent < 100 {
		detail += fmt.Sprintf(" History from the phone: %d%% received so far; older messages appear as it arrives.", h.Percent)
	}
	detail += limitsText(c, time.Now())
	var ban linkflow.Ban
	next := ""
	if !c.Retry.IsZero() {
		next = " Next try: " + c.Retry.Format(whenFormat) + "."
	}
	switch {
	case c.Connected:
		return "connected", detail + " Connected and ready to send and receive."
	case errors.As(c.Err, &ban):
		return "banned", fmt.Sprintf("%s WhatsApp has temporarily banned this account (reason %d), usually for messaging too many people or sending the same message many times. Nothing can be sent until the ban ends; Chatwire reconnects by itself then.%s", detail, ban.Code, next)
	case errors.Is(c.Err, linkflow.ErrReplaced):
		return "replaced", detail + " Another program is using this same WhatsApp link, so WhatsApp dropped this one. To avoid a tug of war that WhatsApp punishes, Chatwire only retries now and then; close the other program." + next
	case errors.Is(c.Err, linkflow.ErrOutdated), errors.Is(c.Err, linkflow.ErrClient):
		return "refused", fmt.Sprintf("%s WhatsApp refused this client (%v). Chatwire retries hourly; updating Chatwire usually fixes this.%s", detail, c.Err, next)
	case c.Err != nil && c.Failures >= manyFailures:
		return "offline", fmt.Sprintf("%s Still not connected after %d tries (%v). Check that this computer is online; Chatwire keeps retrying.%s", detail, c.Failures, c.Err, next)
	case c.Err != nil:
		return "reconnecting", fmt.Sprintf("%s Not connected right now (%v); it keeps retrying.%s", detail, c.Err, next)
	default:
		return "connecting", detail + " Connecting to WhatsApp."
	}
}

const (
	manyFailures = 10
	whenFormat   = "15:04 on Jan 2"
)

func limitsText(c messenger.Connection, now time.Time) string {
	var text string
	if c.Timelock.On(now) {
		text += fmt.Sprintf(" WhatsApp restricts this account from messaging people who have not messaged it until %s (%s); existing conversations still work.", c.Timelock.Ends.Format(whenFormat), c.Timelock.Kind)
	}
	switch a := c.Allowance; {
	case a.Reached(now):
		text += fmt.Sprintf(" WhatsApp's allowance of %d messages to people who have not replied is used up until %s.", a.Total, a.Ends.Format(whenFormat))
	case a.Warned(now):
		text += fmt.Sprintf(" WhatsApp warns that %d of this account's %d messages to people who have not replied are used (resets %s); message fewer new people.", a.Used, a.Total, a.Ends.Format(whenFormat))
	}
	return text
}

func describe(st linker.Status) Report {
	switch st.Phase {
	case linker.ShowingCode:
		intro := "Linking code: %s\n\nThe phone with number +%s may show a WhatsApp notification about linking a device; tapping it leads to where the code is typed. Otherwise open WhatsApp's Linked devices screen:\n"
		if st.Renewed {
			intro = "The earlier code expired, so there is a NEW linking code: %s\n\nShow the user this one instead. On the phone with number +%s, open WhatsApp's Linked devices screen:\n"
		}
		return Report{State: "waiting_for_code", Code: st.Code, Detail: fmt.Sprintf(
			intro+
				"- iPhone: Settings > Linked Devices > Link a Device\n"+
				"- Android: the three-dot menu > Linked devices > Link a device\n"+
				"Then tap \"Link with phone number instead\" and type this code. Chatwire replaces an unused code every few minutes; status shows the current one.",
			st.Code, st.Phone)}
	case linker.ShowingQR:
		return Report{State: "waiting_for_scan", Detail: "Show the user this QR code. " + scanSteps + " " +
			"It changes every 20 seconds; if it stops working, call link_whatsapp again, or link with the phone number instead to avoid scanning."}
	case linker.Linked:
		return Report{State: "linked", LinkedAs: "+" + st.Account.JID.User, Detail: fmt.Sprintf("WhatsApp is linked to +%s.", st.Account.JID.User)}
	case linker.Expired:
		return Report{State: "expired", Detail: "The linking code or QR code expired before it was used. Call link_whatsapp again for a fresh one."}
	case linker.Failed:
		return Report{State: stateFailed, Detail: fmt.Sprintf("Linking failed: %v. Call link_whatsapp to try again.", st.Err)}
	case linker.Starting:
		return Report{State: "starting", Detail: "Contacting WhatsApp."}
	case linker.LoggedOut:
		return Report{State: "logged_out", Detail: "WhatsApp unlinked this computer: it was removed on the phone, or the phone was offline for about two weeks. " +
			"Ask the user for their WhatsApp mobile number with country code and call link_whatsapp to link again."}
	default:
		return Report{State: stateNotLinked, Detail: "WhatsApp is not linked yet. Ask the user for their WhatsApp mobile number with country code and call link_whatsapp with it."}
	}
}

func withPage(ctx context.Context, opts Options, report Report) Report {
	if opts.LinkPage == nil {
		return report
	}
	url, opened, err := opts.LinkPage(ctx)
	switch {
	case err != nil:
		return report
	case opened:
		report.Detail = "A page with the live QR code is now open in the user's browser: " + url + "\n" + scanSteps +
			" The page keeps up as the code changes and says when linking is done. Tell the user, then call whatsapp_status with wait_seconds."
	default:
		report.Detail = "Ask the user to open this page in a browser on this computer, where the live QR code is shown: " + url + "\n" + scanSteps +
			" If there is no browser here (a remote or headless machine), link with the phone number instead."
	}
	report.Page = url
	return report
}

func qrResult(data string, report Report) (*mcp.CallToolResult, Report, error) {
	code, err := qr.Encode(data, qr.M)
	if err != nil {
		return nil, Report{}, fmt.Errorf("mcptools: qr: %w", err)
	}
	return withJSON(report, &mcp.ImageContent{MIMEType: "image/png", Data: code.PNG()}), report, nil
}

const scanSteps = "On the phone, scan it from WhatsApp's Linked devices screen " +
	"(iPhone: Settings > Linked Devices > Link a Device; Android: the three-dot menu > Linked devices > Link a device)."
