package mcpapp

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/mcptools"
	"github.com/PeterStoica/chatwire/internal/setup"
	"github.com/PeterStoica/chatwire/internal/shim"
)

const (
	maxWaitStep    = 120 * time.Second
	defaultLinkFor = 5 * time.Minute
)

const usage = `Chatwire connects AI apps to your WhatsApp. Unofficial; not affiliated with WhatsApp or Meta.

  chatwire setup            add Chatwire to the AI apps on this computer, then link WhatsApp
  chatwire link             link WhatsApp to this computer (a QR page opens in your browser)
  chatwire link --phone +40 721 234 567
                            link with an 8-character code typed on the phone instead
  chatwire status           show whether WhatsApp is linked and connected
  chatwire update           install the newest Chatwire (--check only says whether there is one)
  chatwire setup --remove   take Chatwire out of every AI app again

AI apps start Chatwire by themselves; you never need to leave it running.
Add --json to setup, link or status for output made for programs.
`

type cli struct {
	ctx    context.Context
	state  string
	linger time.Duration
	out    io.Writer
	asJSON bool
	human  bool
}

func terminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (c cli) session() (*mcp.ClientSession, error) {
	executable, build, err := self()
	if err != nil {
		return nil, err
	}
	socket, err := socketPath(c.state)
	if err != nil {
		return nil, err
	}
	launcher := shim.Launcher{
		Socket: socket, Log: filepath.Join(filepath.Dir(c.state), "daemon.log"), Build: build, Executable: executable,
		DaemonArgs: []string{"daemon", "-state", c.state, "-linger", c.linger.String()},
	}
	conn, err := launcher.Connect(c.ctx)
	if err != nil {
		return nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "chatwire-cli", Version: version()}, nil)
	session, err := client.Connect(c.ctx, &mcp.IOTransport{Reader: conn, Writer: conn}, nil)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("chatwire: talk to the background process: %w", err)
	}
	return session, nil
}

func call[T any](ctx context.Context, s *mcp.ClientSession, tool string, args map[string]any) (T, error) {
	var out T
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return out, fmt.Errorf("chatwire: %s: %w", tool, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("chatwire: %s answered in an unexpected form: %w", tool, err)
	}
	return out, nil
}

func (c cli) print(value any, text string) {
	if c.asJSON {
		raw, err := json.Marshal(value)
		if err == nil {
			_, _ = fmt.Fprintln(c.out, string(raw))
		}
		return
	}
	_, _ = fmt.Fprintln(c.out, text)
}

func (c cli) status(wait time.Duration) error {
	s, err := c.session()
	if err != nil {
		return err
	}
	defer s.Close()
	report, err := call[mcptools.Report](c.ctx, s, "whatsapp_status", map[string]any{"wait_seconds": int(min(wait, maxWaitStep).Seconds())})
	if err != nil {
		return err
	}
	c.print(report, report.Detail)
	return nil
}

func (c cli) link(phone string, wait time.Duration) error {
	s, err := c.session()
	if err != nil {
		return err
	}
	defer s.Close()
	args := map[string]any{}
	if phone != "" {
		args["phone_number"] = phone
	}
	report, err := call[mcptools.Report](c.ctx, s, "link_whatsapp", args)
	if err != nil {
		return err
	}
	c.print(report, linkText(report))
	if !linking(report.State) || wait <= 0 {
		if linking(report.State) && !c.asJSON {
			_, _ = fmt.Fprintln(c.out, "Linking goes on in the background. Check with: chatwire status --wait 60")
		}
		return nil
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		report, err = call[mcptools.Report](c.ctx, s, "whatsapp_status", map[string]any{"wait_seconds": int(min(time.Until(deadline), maxWaitStep).Seconds())})
		if err != nil {
			return err
		}
		if !linking(report.State) {
			break
		}
	}
	c.print(report, report.Detail)
	return nil
}

func linking(state string) bool {
	return state == "waiting_for_scan" || state == "waiting_for_code" || state == "starting"
}

func linkText(r mcptools.Report) string {
	switch {
	case r.Code != "":
		return fmt.Sprintf("Your linking code: %s\n\nOn your phone, open WhatsApp, then Linked devices, then Link a device, then \"Link with phone number instead\", and type this code.", r.Code)
	case r.Page != "":
		return "A page with the QR code opened in your browser: " + r.Page + "\n\nOn your phone, open WhatsApp, then Linked devices, then Link a device, and scan it."
	}
	return r.Detail
}

func (c cli) setupCommand(args []string, in io.Reader) error {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	flags.SetOutput(c.out)
	asJSON := flags.Bool("json", false, "print results as JSON")
	yes := flags.Bool("yes", false, "do not ask before changing the apps' settings")
	remove := flags.Bool("remove", false, "take Chatwire out of the apps instead")
	only := flags.String("client", "", "only these apps, comma separated: "+strings.Join(clientIDs(), ", "))
	noLink := flags.Bool("no-link", false, "do not link WhatsApp afterwards")
	if err := flags.Parse(args); err != nil {
		return err
	}
	c.asJSON = c.asJSON || *asJSON
	env, err := setup.System()
	if err != nil {
		return err
	}
	executable, _, err := self()
	if err != nil {
		return err
	}
	chosen, err := choose(setup.Clients(env), *only)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		c.print(map[string]any{"results": []setup.Result{}}, "No AI apps found on this computer. Name one with --client, for example: chatwire setup --client claude-desktop")
		return nil
	}
	if c.human && !*yes && !confirm(c.out, in, chosen, *remove) {
		_, _ = fmt.Fprintln(c.out, "Nothing changed.")
		return nil
	}
	results := setup.Apply(c.ctx, env, chosen, executable, *remove)
	c.print(map[string]any{"command": executable, "results": results}, resultText(results, *remove))
	if *remove || *noLink {
		return nil
	}
	wait := time.Duration(0)
	if c.human {
		wait = defaultLinkFor
	}
	return c.linkIfNeeded(wait)
}

func (c cli) linkIfNeeded(wait time.Duration) error {
	s, err := c.session()
	if err != nil {
		return err
	}
	report, err := call[mcptools.Report](c.ctx, s, "whatsapp_status", map[string]any{})
	_ = s.Close()
	if err != nil {
		return err
	}
	if report.State == "linked" {
		c.print(report, "\n"+report.Detail)
		return nil
	}
	if !c.asJSON {
		_, _ = fmt.Fprintln(c.out, "\nNow link your WhatsApp.")
	}
	return c.link("", wait)
}

func clientIDs() []string {
	var ids []string
	for _, cl := range setup.Clients(setup.Env{LookPath: func(string) (string, error) { return "", errors.New("unused") }}) {
		ids = append(ids, cl.ID)
	}
	return ids
}

func choose(all []setup.Client, only string) ([]setup.Client, error) {
	if only == "" {
		return slices.DeleteFunc(all, func(cl setup.Client) bool { return !cl.Found() }), nil
	}
	var out []setup.Client
	for _, id := range strings.Split(only, ",") {
		id = strings.TrimSpace(id)
		i := slices.IndexFunc(all, func(cl setup.Client) bool { return cl.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("chatwire: no app called %q; the apps are: %s", id, strings.Join(clientIDs(), ", "))
		}
		out = append(out, all[i])
	}
	return out, nil
}

func confirm(out io.Writer, in io.Reader, chosen []setup.Client, remove bool) bool {
	names := make([]string, 0, len(chosen))
	for _, cl := range chosen {
		names = append(names, cl.Name)
	}
	verb := "Add Chatwire to"
	if remove {
		verb = "Take Chatwire out of"
	}
	_, _ = fmt.Fprintf(out, "Found: %s\n%s these? [Y/n] ", strings.Join(names, ", "), verb)
	var answer string
	_, _ = fmt.Fscanln(in, &answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes"
}

func resultText(results []setup.Result, remove bool) string {
	var b strings.Builder
	var restart []string
	for _, r := range results {
		mark := "✓"
		if r.Outcome == setup.Failed || r.Outcome == setup.Skipped {
			mark = "!"
		}
		line := fmt.Sprintf("  %s %s: %s", mark, r.Name, r.Outcome)
		if r.Detail != "" {
			line += " (" + r.Detail + ")"
		}
		b.WriteString(line + "\n")
		if r.Outcome == setup.Added || r.Outcome == setup.Updated || r.Outcome == setup.Removed {
			restart = append(restart, r.Name)
		}
	}
	if len(restart) > 0 {
		verb := "see"
		if remove {
			verb = "stop seeing"
		}
		fmt.Fprintf(&b, "Restart %s (or open a new session) to %s the WhatsApp tools.", strings.Join(restart, ", "), verb)
	}
	return strings.TrimRight(b.String(), "\n")
}
