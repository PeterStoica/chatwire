package mcptools_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/goleak"
	"google.golang.org/protobuf/proto"
	"rsc.io/qr"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/linker"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/mcptools"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeappstate"
	"github.com/PeterStoica/chatwire/internal/testkit/fakecdn"
	"github.com/PeterStoica/chatwire/internal/testkit/fakedevice"
	"github.com/PeterStoica/chatwire/internal/testkit/fakegroups"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
	"github.com/PeterStoica/chatwire/internal/testkit/fakerelay"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeworld"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type claude struct {
	t       *testing.T
	session *mcp.ClientSession
}

func connect(t *testing.T, w *fakeworld.World, cdn ...*http.Client) (claude, *linker.Linker) {
	t.Helper()
	return connectWith(t, w, mcptools.Options{}, cdn...)
}

func connectWith(t *testing.T, w *fakeworld.World, opts mcptools.Options, cdn ...*http.Client) (claude, *linker.Linker) {
	t.Helper()
	cfg := linkflow.Config{
		Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: w.Random, Now: time.Now,
		Save: func(linkflow.Linked) error { return nil }, KeepAlive: -1,
	}
	l := linker.New(cfg, nil)
	w.Screen = func() string { return l.Status().QR }
	online := cfg
	online.Random = rand.Reader
	var httpClient *http.Client
	if len(cdn) > 0 {
		httpClient = cdn[0]
	}
	messages, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	m := messenger.New(online, httpClient, func(client.State) error { return nil }, messages)
	l.WhenLinked(func(linked linkflow.Linked) { m.Start(t.Context(), client.State{Linked: linked}) })
	m.WhenLoggedOut(l.LoggedOut)
	opts.MediaDir = t.TempDir()
	server := mcptools.NewServer(&mcp.Implementation{Name: "whatsapp", Version: "test"}, l, m, opts)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "claude", Version: "test"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		_ = serverSession.Wait()
		l.Close()
		m.Close()
		_ = messages.Close()
	})
	return claude{t: t, session: session}, l
}

func (c claude) call(name string, args map[string]any) (mcptools.Report, *mcp.CallToolResult) {
	c.t.Helper()
	return callAs[mcptools.Report](c, name, args)
}

func callAs[T any](c claude, name string, args map[string]any) (T, *mcp.CallToolResult) {
	c.t.Helper()
	var report T
	result, err := c.session.CallTool(c.t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.t.Fatal(err)
	}
	if result.IsError {
		c.t.Fatalf("%s failed: %v", name, result.Content)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		c.t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		c.t.Fatal(err)
	}
	return report, result
}

func fromText[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	var out T
	if err := json.Unmarshal([]byte(textOf(result)), &out); err != nil {
		t.Fatalf("the text content is not the result as JSON: %v\n%s", err, textOf(result))
	}
	return out
}

func textOf(result *mcp.CallToolResult) string {
	var b strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func TestLinkingByPhoneNumberFromClaudesSide(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(31)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()))
		c, _ := connect(t, w)
		instructions := c.session.InitializeResult().Instructions
		if instructions != mcptools.Instructions {
			t.Fatalf("instructions = %q", instructions)
		}
		tools, err := c.session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		named := map[string]bool{}
		writes := map[string]bool{"send_whatsapp_message": true, "send_whatsapp_file": true, "change_whatsapp_message": true, "manage_whatsapp_group": true, "link_whatsapp": true}
		for _, tool := range tools.Tools {
			named[tool.Name] = true
			if !portableName.MatchString(tool.Name) || len(tool.Description) > 1024 {
				t.Fatalf("%s: a name or description other platforms refuse", tool.Name)
			}
			if a := tool.Annotations; a == nil || a.Title == "" || a.ReadOnlyHint == writes[tool.Name] {
				t.Fatalf("%s: annotations %+v; only reads may say they change nothing", tool.Name, a)
			}
			raw, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			if problem := unportable(schema); problem != "" {
				t.Fatalf("%s input schema: %s", tool.Name, problem)
			}
		}
		for _, name := range []string{"whatsapp_status", "link_whatsapp", "read_whatsapp_messages", "get_whatsapp_media", "change_whatsapp_message"} {
			if !strings.Contains(instructions, name) || !named[name] {
				t.Fatalf("the instructions and the tools disagree about %s", name)
			}
		}
		if report, _ := c.call("whatsapp_status", nil); report.State != "not_linked" || !strings.Contains(report.Detail, "mobile number") {
			t.Fatalf("before linking: %+v", report)
		}
		report, result := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		if report.State != "waiting_for_code" || len(report.Code) != 9 || !strings.Contains(textOf(result), report.Code) || !strings.Contains(textOf(result), "+40700000000") {
			t.Fatalf("link_whatsapp = %+v\n%s", report, textOf(result))
		}
		again, _ := c.call("link_whatsapp", map[string]any{"phone_number": "40700000000"})
		if again.Code != report.Code {
			t.Fatalf("asking again changed the code from %s to %s", report.Code, again.Code)
		}
		go func() {
			time.Sleep(30 * time.Second)
			w.Type(report.Code)
		}()
		start := time.Now()
		done, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 90})
		if done.State != "linked" || done.LinkedAs != "+40700000000" {
			t.Fatalf("after typing: %+v", done)
		}
		if waited := time.Since(start); waited != 30*time.Second {
			t.Fatalf("whatsapp_status answered after %s, want the moment linking finished (30s)", waited)
		}
		if linked, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"}); linked.State != "linked" {
			t.Fatalf("link_whatsapp after linking: %+v", linked)
		}
	})
}

func TestLinkingByQRFromClaudesSide(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(32)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(w.QRPairing(15*time.Second), w.Login(fakeworld.Success()))
		pages := 0
		c, l := connectWith(t, w, mcptools.Options{LinkPage: func(context.Context) (string, bool, error) {
			pages++
			return "http://127.0.0.1:1/0123/", true, nil
		}})
		report, result := c.call("link_whatsapp", nil)
		if pages != 1 || report.Page != "http://127.0.0.1:1/0123/" || !strings.Contains(report.Detail, "now open in the user's browser: http://127.0.0.1:1/0123/") ||
			!strings.Contains(report.Detail, "Linked Devices") {
			t.Fatalf("the QR page: %d opened, %+v", pages, report)
		}
		var image *mcp.ImageContent
		for _, content := range result.Content {
			if img, ok := content.(*mcp.ImageContent); ok {
				image = img
			}
		}
		if report.State != "waiting_for_scan" || image == nil || image.MIMEType != "image/png" {
			t.Fatalf("link_whatsapp = %+v, image %v", report, image != nil)
		}
		want, err := qr.Encode(l.Status().QR, qr.M)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(image.Data, want.PNG()) {
			t.Fatal("the image is not the QR code the phone scans")
		}
		done, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		if done.State != "linked" {
			t.Fatalf("after scanning: %+v", done)
		}
	})
}

func TestReadOnlyLeavesOutEveryTool(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(36)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := connectWith(t, w, mcptools.Options{ReadOnly: true})
		if !strings.HasSuffix(c.session.InitializeResult().Instructions, mcptools.ReadOnlyNote) {
			t.Fatal("the instructions do not say it is read-only")
		}
		tools, err := c.session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range tools.Tools {
			if !tool.Annotations.ReadOnlyHint && tool.Name != "link_whatsapp" {
				t.Fatalf("read-only mode offers %s", tool.Name)
			}
			names = append(names, tool.Name)
		}
		if !slices.Contains(names, "read_whatsapp_messages") || slices.Contains(names, "send_whatsapp_message") {
			t.Fatalf("tools = %v", names)
		}
	})
}

func TestWaitingGivesUpAfterTheRequestedTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(33)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(w.CodePairing())
		c, _ := connect(t, w)
		if report, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"}); report.State != "waiting_for_code" {
			t.Fatalf("link_whatsapp = %+v", report)
		}
		start := time.Now()
		report, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 45})
		if report.State != "waiting_for_code" || time.Since(start) != 45*time.Second {
			t.Fatalf("after %s: %+v", time.Since(start), report)
		}
		start = time.Now()
		if report, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 999}); time.Since(start) != 50*time.Second || report.State != "waiting_for_code" {
			t.Fatalf("an oversized wait lasted %s: %+v", time.Since(start), report)
		}
	})
}

func TestWaitingEndsWhenTheCodeIsReplaced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(35)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(func(fc *fakeworld.Conn) {
			w.OfferPairing(fc)
			for {
				w.AnswerHello(fc)
			}
		})
		c, _ := connect(t, w)
		first, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		start := time.Now()
		report := first
		for range 5 {
			if report, _ = c.call("whatsapp_status", map[string]any{"wait_seconds": 50}); report.Code != first.Code {
				break
			}
		}
		if time.Since(start) != 195*time.Second || report.Code == first.Code || !strings.Contains(report.Detail, "NEW linking code: "+report.Code) {
			t.Fatalf("after %s: %+v", time.Since(start), report)
		}
	})
}

func TestInvalidPhoneNumberAsksForTheInternationalForm(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(34)
		if err != nil {
			t.Fatal(err)
		}
		c, l := connect(t, w)
		report, _ := c.call("link_whatsapp", map[string]any{"phone_number": "0721 234 567"})
		if report.State != "invalid_phone_number" || !strings.Contains(report.Detail, "country code") || l.Status().Phase != linker.Unlinked {
			t.Fatalf("link_whatsapp = %+v, linker %+v", report, l.Status())
		}
	})
}

func TestWaitingAnswersAtOnceWhenNothingIsInProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(35)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()))
		c, _ := connect(t, w)
		start := time.Now()
		if report, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 30}); report.State != "not_linked" || time.Since(start) != 0 {
			t.Fatalf("not linked: waited %s for %+v", time.Since(start), report)
		}
		c.call("link_whatsapp", nil)
		if report, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 30}); report.State != "linked" {
			t.Fatalf("after scanning: %+v", report)
		}
		start = time.Now()
		if report, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 30}); report.State != "linked" || time.Since(start) != 0 {
			t.Fatalf("linked: waited %s for %+v", time.Since(start), report)
		}
	})
}

func TestSendingFromClaudeAfterLinking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(34)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		if err := ourPhone.Upload(keys, 3); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		var mu sync.Mutex
		delivered := map[node.JID][]node.Node{}
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: make(chan node.Node),
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				delivered[device] = append(delivered[device], stanza)
			},
		}))
		c, _ := connect(t, w)
		if report, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "me", "text": "too early"}); report.State != "not_linked" {
			t.Fatalf("before linking: %+v", report)
		}
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		if done, _ := c.call("whatsapp_status", map[string]any{"wait_seconds": 60}); done.State != "linked" {
			t.Fatalf("linking: %+v", done)
		}
		for _, tt := range []struct {
			args  map[string]any
			state string
		}{
			{map[string]any{"to": "0721 234 567", "text": "hi"}, "unknown_recipient"},
			{map[string]any{"to": "me", "text": "   "}, "empty_message"},
		} {
			if report, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", tt.args); report.State != tt.state {
				t.Fatalf("send %v = %+v, want %s", tt.args, report, tt.state)
			}
		}
		sent, result := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "Me", "text": "note to self"})
		if sent.State != "sent" || sent.To != "me (+40700000000)" || !strings.HasPrefix(sent.ID, "3EB0") || !strings.Contains(textOf(result), "Sent to me (+40700000000)") {
			t.Fatalf("send = %+v", sent)
		}
		mu.Lock()
		copies := delivered[account]
		mu.Unlock()
		if len(copies) != 1 {
			t.Fatalf("our phone got %d copies", len(copies))
		}
		in, m, err := ourPhone.Receive(copies[0])
		if err != nil || in.Chat != account || in.ID != sent.ID || m.GetDeviceSentMessage().GetDestinationJid() != account.String() || m.GetDeviceSentMessage().GetMessage().GetConversation() != "note to self" {
			t.Fatalf("our phone read %v in %v: %v", m, in.Chat, err)
		}
		synctest.Sleep(5 * time.Second)
		again, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "Me", "text": "note to self"})
		if again.State != "sent" || again.ID != sent.ID || !strings.Contains(again.Detail, "already sent 5s ago") {
			t.Fatalf("a repeated call = %+v, want the first send's result", again)
		}
		synctest.Sleep(30 * time.Second)
		later, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "Me", "text": "note to self"})
		mu.Lock()
		copies = delivered[account]
		mu.Unlock()
		if later.State != "sent" || later.ID == sent.ID || len(copies) != 2 {
			t.Fatalf("the same text half a minute later = %+v with %d copies, want a second message", later, len(copies))
		}
	})
}

func TestReadingFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(35)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range []*fakedevice.Device{ourPhone, bobPhone} {
			if err := d.Upload(keys, 3); err != nil {
				t.Fatal(err)
			}
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		inbox := make(chan node.Node)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {},
		}))
		c, _ := connect(t, w)
		if report, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil); report.State != "not_linked" {
			t.Fatalf("before linking: %+v", report)
		}
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		if status, _ := c.call("whatsapp_status", nil); status.State != "linked" || status.Connection != "connected" || !strings.Contains(status.Detail, "ready to send") {
			t.Fatalf("status = %+v", status)
		}
		if empty, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil); empty.State != "ok" || len(empty.Messages) != 0 || !strings.Contains(empty.Detail, "No messages") {
			t.Fatalf("nothing received yet: %+v", empty)
		}
		push := func(from *fakedevice.Device, to node.JID, text string) {
			t.Helper()
			out, err := from.Send(keys, devices, to, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(from.JID, map[node.JID]string{bob: "Bob", account: "Me"}[from.JID], time.Now(), out)[w.Phone.JID]
		}
		push(bobPhone, account, "are you there?")
		push(ourPhone, bob, "yes, on my way")
		synctest.Wait()
		all, result := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil)
		if len(all.Messages) != 2 {
			t.Fatalf("read %d messages: %+v", len(all.Messages), all)
		}
		first, second := all.Messages[0], all.Messages[1]
		if first.Text != "are you there?" || first.From != "Bob (+40722222222)" || first.Chat != "Bob (+40722222222)" || first.Name != "Bob" || first.FromMe || first.Kind != "text" {
			t.Fatalf("bob's message = %+v", first)
		}
		if second.Text != "yes, on my way" || second.Chat != "Bob (+40722222222)" || !second.FromMe {
			t.Fatalf("our phone's message = %+v", second)
		}
		if plain := fromText[mcptools.ReadReport](t, result); len(plain.Messages) != 2 || plain.Messages[0].ID != first.ID || plain.Messages[1].ID != second.ID || plain.Messages[1].Text != "yes, on my way" {
			t.Fatalf("text = %s", textOf(result))
		}
		latest, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"limit": 1})
		if len(latest.Messages) != 1 || latest.Messages[0].Text != "yes, on my way" || !strings.Contains(latest.Detail, "before="+latest.Messages[0].ID) {
			t.Fatalf("limit 1 = %+v", latest)
		}
		for _, tt := range []struct {
			args  map[string]any
			state string
			count int
		}{
			{map[string]any{"chat": "+40 733 333 333"}, "ok", 0},
			{map[string]any{"chat": "+40 722 222 222"}, "ok", 2},
			{map[string]any{"chat": "bob"}, "ok", 2},
			{map[string]any{"chat": "BOB", "query": "THERE"}, "ok", 1},
			{map[string]any{"query": "nowhere"}, "ok", 0},
			{map[string]any{"before": latest.Messages[0].ID}, "ok", 1},
			{map[string]any{"before": latest.Messages[0].ID, "chat": "bob"}, "ok", 1},
			{map[string]any{"before": first.ID}, "ok", 0},
			{map[string]any{"before": latest.Messages[0].Time}, "ok", 0},
			{map[string]any{"chat": "nobody"}, "unknown_chat", 0},
			{map[string]any{"before": "yesterday"}, "invalid_before", 0},
			{map[string]any{"limit": -5}, "invalid_limit", 0},
		} {
			if got, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", tt.args); got.State != tt.state || len(got.Messages) != tt.count {
				t.Fatalf("read %v = %+v", tt.args, got)
			}
		}
	})
}

func TestGroupsFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(36)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 3); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		members := []groups.Participant{{JID: account}, {JID: bob}}
		family := groups.Group{JID: node.JID{User: "120363000000000011", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
			Participants: []groups.Participant{{JID: account, Admin: true}, {JID: bob}}, Description: "Sunday lunch planning"}
		trip := groups.Group{JID: node.JID{User: "120363000000000012", Server: node.ServerGroup}, Subject: "Family Trip 2026", Created: time.Unix(1700000000, 0), Participants: members}
		var mu sync.Mutex
		delivered := map[node.JID][]node.Node{}
		inbox := make(chan node.Node)
		groupsServer := fakegroups.New(family, trip)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Groups: groupsServer,
			Members: func(node.JID) []node.JID { return []node.JID{bob, w.Phone.JID} },
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				delivered[device] = append(delivered[device], stanza)
			},
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		chats, chatsResult := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", map[string]any{"limit": 1})
		if len(chats.Chats) != 1 || chats.Chats[0].LastActive != "" || !chats.Chats[0].Group || !fromText[mcptools.ChatsReport](t, chatsResult).Chats[0].Group {
			t.Fatalf("groups without messages = %+v\n%s", chats, textOf(chatsResult))
		}

		listed, result := callAs[mcptools.GroupsReport](c, "list_whatsapp_groups", nil)
		if listed.State != "ok" || len(listed.Groups) != 2 || listed.Groups[1].Name != "Family Trip 2026" || listed.Groups[0].Participants != 2 || fromText[mcptools.GroupsReport](t, result).Groups[0].ID != "120363000000000011@g.us" {
			t.Fatalf("groups = %+v\n%s", listed, textOf(result))
		}
		roster, rosterText := callAs[mcptools.GroupsReport](c, "list_whatsapp_groups", map[string]any{"group": "FAMILY"})
		if len(roster.Groups) != 1 || roster.Groups[0].Name != "Family" || roster.Groups[0].Description != "Sunday lunch planning" || len(roster.Groups[0].Members) != 2 ||
			!roster.Groups[0].Members[0].Admin || roster.Groups[0].Members[1].Admin || fromText[mcptools.GroupsReport](t, rosterText).Groups[0].Members[0].Name != "me (+40700000000)" ||
			roster.Groups[0].Description != "Sunday lunch planning" {
			t.Fatalf("family members = %+v\n%s", roster, textOf(rosterText))
		}
		if partial, _ := callAs[mcptools.GroupsReport](c, "list_whatsapp_groups", map[string]any{"group": "trip"}); len(partial.Groups) != 1 || partial.Groups[0].Name != "Family Trip 2026" || len(partial.Groups[0].Members) != 2 {
			t.Fatalf("trip members = %+v", partial)
		}
		if unknown, _ := callAs[mcptools.GroupsReport](c, "list_whatsapp_groups", map[string]any{"group": "golf"}); unknown.State != "unknown_group" {
			t.Fatalf("an unknown group = %+v", unknown)
		}

		for _, tt := range []struct {
			to, state, name string
		}{
			{"family", "sent", "Family"},
			{"trip", "sent", "Family Trip 2026"},
			{"120363000000000012@g.us", "sent", "Family Trip 2026"},
			{"fam", "ambiguous_recipient", ""},
			{"nobody", "unknown_recipient", ""},
		} {
			report, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": tt.to, "text": "hi all"})
			if report.State != tt.state || report.To != tt.name {
				t.Fatalf("send to %q = %+v", tt.to, report)
			}
		}
		mu.Lock()
		toBob := delivered[bob]
		mu.Unlock()
		if len(toBob) != 3 {
			t.Fatalf("bob got %d group messages", len(toBob))
		}
		in, m, err := bobPhone.Receive(toBob[0])
		if err != nil || in.Chat != family.JID || m.GetConversation() != "hi all" {
			t.Fatalf("bob read %v in %v: %v", m, in.Chat, err)
		}

		key, err := signal.NewSenderKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		distribution, err := key.Distribution()
		if err != nil {
			t.Fatal(err)
		}
		skdm, err := bobPhone.EncryptFor(keys, w.Phone.JID, message.SenderKeyDistribution(family.JID, distribution))
		if err != nil {
			t.Fatal(err)
		}
		padded, err := message.Encode(rand.Reader, &wire.Message{Conversation: new("dinner at 8")})
		if err != nil {
			t.Fatal(err)
		}
		ciphertext, err := key.Encrypt(rand.Reader, padded)
		if err != nil {
			t.Fatal(err)
		}
		enc := func(kind string, raw []byte) node.Node {
			return node.Node{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text(kind)}}, Bytes: raw}
		}
		pairwiseType := map[signal.MessageType]string{signal.TypeMessage: "msg", signal.TypePreKeyMessage: "pkmsg"}[skdm.Ciphertext.Type]
		inbox <- node.Node{Tag: "message", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(family.JID)}, {Key: "participant", Value: node.Address(bob)}, {Key: "type", Value: node.Text("text")},
			{Key: "id", Value: node.Text("3EB0DINNER")}, {Key: "t", Value: node.Text("1790000000")}, {Key: "notify", Value: node.Text("Bob")},
		}, Children: []node.Node{enc(pairwiseType, skdm.Ciphertext.Bytes), enc("skmsg", ciphertext)}}
		synctest.Wait()
		read, readResult := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil)
		if len(read.Messages) != 4 || read.Messages[3].Chat != "Family" || read.Messages[3].Text != "dinner at 8" || fromText[mcptools.ReadReport](t, readResult).Messages[3].From != "Bob (+40722222222)" {
			t.Fatalf("read = %+v\n%s", read, textOf(readResult))
		}
		for i, chat := range []string{"Family", "Family Trip 2026", "Family Trip 2026"} {
			if m := read.Messages[i]; !m.FromMe || m.Chat != chat || m.Text != "hi all" {
				t.Fatalf("our message %d = %+v", i, m)
			}
		}
		if family, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "family"}); len(family.Messages) != 2 {
			t.Fatalf("the Family chat = %+v", family)
		}
		created := groups.Group{JID: node.JID{User: "120363000000000013", Server: node.ServerGroup}, Subject: "Book Club", Created: time.Unix(1700000000, 0), Participants: members}
		groupsServer.Add(created)
		inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(created.JID)}, {Key: "type", Value: node.Text("w:gp2")}, {Key: "id", Value: node.Text("G1")}, {Key: "t", Value: node.Text("1790000100")},
		}, Children: []node.Node{{Tag: "add", Children: []node.Node{{Tag: "participant", Attrs: []node.Attr{{Key: "jid", Value: node.Address(account)}}}}}}}
		synctest.Wait()
		if joined, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "book club", "text": "hello club"}); joined.State != "sent" || joined.To != "Book Club" {
			t.Fatalf("a group made after connecting = %+v", joined)
		}
		if sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "family", "text": "@Bob see you at 8"}); sent.State != "sent" {
			t.Fatalf("a mention = %+v", sent)
		}
		mu.Lock()
		last := delivered[bob][len(delivered[bob])-1]
		mu.Unlock()
		if _, m, err := bobPhone.Receive(last); err != nil || m.GetExtendedTextMessage().GetText() != "@40722222222 see you at 8" ||
			!slices.Equal(m.GetExtendedTextMessage().GetContextInfo().GetMentionedJid(), []string{bob.String()}) {
			t.Fatalf("bob got %v: %v", m, err)
		}
	})
}

func TestMediaFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(37)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 3); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		var mu sync.Mutex
		files := map[string][]byte{}
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if file, ok := files[req.URL.Path]; ok {
				_, _ = rw.Write(file)
				return
			}
			http.NotFound(rw, req)
		}))
		inbox := make(chan node.Node)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {},
		}))
		c, _ := connect(t, w, cdn.Client())
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()

		upload := func(path string, kind media.Type, content []byte) (*wire.Message, []byte) {
			t.Helper()
			mediaKey := make([]byte, 32)
			if _, err := rand.Read(mediaKey); err != nil {
				t.Fatal(err)
			}
			sealed, err := media.Encrypt(mediaKey, kind, content)
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			files[path] = sealed.File
			mu.Unlock()
			if kind == media.Image {
				return &wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: new(path), MediaKey: mediaKey, FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:], Mimetype: new("image/png"), Caption: new("the view")}}, content
			}
			return &wire.Message{AudioMessage: &wire.Message_AudioMessage{DirectPath: new(path), MediaKey: mediaKey, FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:], Mimetype: new("audio/ogg; codecs=opus"), Ptt: new(true)}}, content
		}
		photoMessage, photo := upload("/v/photo.enc", media.Image, bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 1000))
		voiceMessage, voice := upload("/v/voice.enc", media.Voice, bytes.Repeat([]byte("OggS"), 800))
		largest, _ := upload("/v/largest.enc", media.Image, make([]byte, 4<<20))
		tooLarge, _ := upload("/v/too-large.enc", media.Image, make([]byte, 4<<20+1))
		for _, m := range []*wire.Message{photoMessage, voiceMessage, {Conversation: new("just text")}, largest, tooLarge} {
			out, err := bobPhone.Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
		}
		synctest.Wait()
		read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil)
		if len(read.Messages) != 5 || read.Messages[0].Kind != "image" || read.Messages[1].Kind != "ptt" ||
			!read.Messages[0].Media || read.Messages[0].Text != "the view" || read.Messages[2].Media {
			t.Fatalf("read = %+v", read.Messages)
		}
		image, result := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": read.Messages[0].ID})
		var shown *mcp.ImageContent
		for _, content := range result.Content {
			if img, ok := content.(*mcp.ImageContent); ok {
				shown = img
			}
		}
		if image.State != "ok" || image.Type != "image" || image.Caption != "the view" || !strings.HasSuffix(image.Detail, "Caption: the view") || shown == nil || shown.MIMEType != "image/png" || !bytes.Equal(shown.Data, photo) {
			t.Fatalf("photo = %+v, shown %v", image, shown != nil)
		}
		saved, _ := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": read.Messages[1].ID})
		if saved.State != "ok" || saved.Path == "" || saved.Size != len(voice) || filepath.Ext(saved.Path) != ".ogg" || strings.Contains(saved.Detail, "Caption") {
			t.Fatalf("voice note = %+v", saved)
		}
		if onDisk, err := os.ReadFile(saved.Path); err != nil || !bytes.Equal(onDisk, voice) {
			t.Fatalf("saved file: %d bytes, %v", len(onDisk), err)
		}
		if info, err := os.Stat(saved.Path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("saved file mode: %v, %v", info.Mode(), err)
		}
		for _, tt := range []struct {
			id    string
			saved bool
		}{{read.Messages[3].ID, false}, {read.Messages[4].ID, true}} {
			report, result := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": tt.id})
			inline := slices.ContainsFunc(result.Content, func(content mcp.Content) bool { _, ok := content.(*mcp.ImageContent); return ok })
			if report.State != "ok" || inline == tt.saved || (report.Path != "") != tt.saved {
				t.Fatalf("image of %d bytes: inline %v, report %+v", report.Size, inline, report)
			}
		}
		for _, tt := range []struct {
			id, state string
		}{{read.Messages[2].ID, "no_media"}, {"3EB0NOPE", "unknown_message"}} {
			if report, _ := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": tt.id}); report.State != tt.state {
				t.Fatalf("media of %s = %+v", tt.id, report)
			}
		}
	})
}

func TestHistoryFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(39)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		carol := node.JID{User: "40733333333", Server: node.ServerUser}
		carolLID := node.JID{User: "88123456789", Server: node.ServerLID}
		club := node.JID{User: "120363000000000031", Server: node.ServerGroup}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		carolPhone, err := fakedevice.New(rand.Reader, carol)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range []*fakedevice.Device{ourPhone, carolPhone} {
			if err := d.Upload(keys, 3); err != nil {
				t.Fatal(err)
			}
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(carol, fakeusync.Device{ID: 0})
		var mu sync.Mutex
		delivered := map[node.JID][]node.Node{}
		inbox := make(chan node.Node)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox,
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				delivered[device] = append(delivered[device], stanza)
			},
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		if empty, _ := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil); empty.State != "ok" || len(empty.Chats) != 0 || !strings.Contains(empty.Detail, "History from the phone") {
			t.Fatalf("before history: %+v", empty)
		}

		said := func(chat, author string, fromMe bool, id string, at uint64, text string) *wire.HistorySyncMsg {
			key := &wire.MessageKey{RemoteJid: new(chat), FromMe: new(fromMe), Id: new(id)}
			if author != "" {
				key.Participant = new(author)
			}
			return &wire.HistorySyncMsg{Message: &wire.MessageInfo{Key: key, Message: &wire.Message{Conversation: new(text)}, MessageTimestamp: new(at)}}
		}
		agreed := said(bob.String(), "", true, "3EB0B2", 1790000200, "Sigur, la 10")
		agreed.Message.Reactions = []*wire.Reaction{{Key: &wire.MessageKey{FromMe: new(false), Id: new("R1")}, Text: new("👍"), SenderTimestampMs: new(int64(1790000210000))}}
		stranger := said(club.String(), "40766666666@s.whatsapp.net", false, "3EB0C2", 1790000301, "hi, new here")
		stranger.Message.PushName = new("Stranger")
		sync := &wire.HistorySync{
			SyncType: wire.HistorySync_INITIAL_BOOTSTRAP.Enum(),
			Progress: new(uint32(40)),
			Conversations: []*wire.Conversation{
				{Id: new(bob.String()), LastMsgTimestamp: new(uint64(1790000200)), Messages: []*wire.HistorySyncMsg{
					said(bob.String(), "", false, "3EB0B1", 1790000100, "Ce mai faci? Mâine la cafea?"),
					agreed,
				}},
				{Id: new(club.String()), Name: new("Book Club"), LastMsgTimestamp: new(uint64(1790000300)), Messages: []*wire.HistorySyncMsg{
					said(club.String(), carolLID.String(), false, "3EB0C1", 1790000300, "Next book: Dune"),
					stranger,
					said(club.String(), "40777777777@s.whatsapp.net", false, "3EB0C3", 1790000302, "no name at all"),
				}},
			},
			Pushnames:                []*wire.Pushname{{Id: new(bob.String()), Pushname: new("Bobby")}},
			PhoneNumberToLidMappings: []*wire.PhoneNumberToLIDMapping{{PnJid: new(carol.String()), LidJid: new(carolLID.String())}},
			InlineContacts:           []*wire.InlineContact{{PnJid: new(carol.String()), LidJid: new(carolLID.String()), FullName: new("Carol Mihai"), FirstName: new("Carol")}},
		}
		raw, err := proto.Marshal(sync)
		if err != nil {
			t.Fatal(err)
		}
		var compressed bytes.Buffer
		zw := zlib.NewWriter(&compressed)
		if _, err := zw.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		out, err := ourPhone.SendPeer(keys, w.Phone.JID, &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
			Type: wire.Message_ProtocolMessage_HISTORY_SYNC_NOTIFICATION.Enum(),
			HistorySyncNotification: &wire.Message_HistorySyncNotification{
				SyncType: wire.Message_INITIAL_BOOTSTRAP.Enum(), InitialHistBootstrapInlinePayload: compressed.Bytes(),
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
		inbox <- fakerelay.DeliverPeer(ourPhone.JID, time.Now(), out)
		synctest.Wait()
		if _, status := c.call("whatsapp_status", nil); !strings.Contains(textOf(status), "History from the phone: 40% received so far") {
			t.Fatalf("status during history sync: %s", textOf(status))
		}

		chats, result := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil)
		if chats.State != "ok" || len(chats.Chats) != 2 || chats.Chats[0].Name != "Book Club" || !chats.Chats[0].Group ||
			chats.Chats[1].Name != "Bobby (+40722222222)" || chats.Chats[1].LastActive != "2026-09-21T14:16:40Z" || fromText[mcptools.ChatsReport](t, result).Chats[0].ID != chats.Chats[0].ID {
			t.Fatalf("chats = %+v\n%s", chats, textOf(result))
		}
		club1, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "book club"})
		if len(club1.Messages) != 3 || club1.Messages[0].From != "Carol Mihai (+40733333333)" || club1.Messages[0].Text != "Next book: Dune" ||
			club1.Messages[1].From != "Stranger (+40766666666)" || club1.Messages[2].From != "+40777777777" {
			t.Fatalf("book club = %+v", club1)
		}
		fromCarol, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "book club", "from": "carol"})
		if len(fromCarol.Messages) != 1 || fromCarol.Messages[0].Text != "Next book: Dune" {
			t.Fatalf("carol's messages, sent under her lid = %+v", fromCarol)
		}
		for args, state := range map[string]string{"book club": "invalid_sender", "nobody at all": "unknown_sender"} {
			if refused, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"from": args}); refused.State != state {
				t.Fatalf("from %q = %+v, want %s", args, refused, state)
			}
		}
		if mixed, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"from": "carol", "unread": true}); mixed.State != "invalid_unread" {
			t.Fatalf("from with unread = %+v", mixed)
		}
		found, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"query": "maine cafea"})
		if len(found.Messages) != 1 || found.Messages[0].ID != "3EB0B1" || found.Messages[0].Chat != "Bobby (+40722222222)" {
			t.Fatalf("search = %+v", found)
		}
		if mine, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"from": "me"}); len(mine.Messages) != 1 || !mine.Messages[0].FromMe {
			t.Fatalf("my messages = %+v", mine)
		}
		withBob, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bobby"})
		if len(withBob.Messages) != 2 || !withBob.Messages[1].FromMe || withBob.Messages[1].From != "me (+40700000000)" || withBob.Messages[1].Text != "Sigur, la 10" || !slices.Equal(withBob.Messages[1].Reactions, []string{"👍 Bobby"}) {
			t.Fatalf("with bob = %+v", withBob)
		}
		sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "caro", "text": "Dune it is"})
		if sent.State != "sent" || sent.To != "Carol Mihai (+40733333333)" {
			t.Fatalf("send to carol = %+v", sent)
		}
		mu.Lock()
		toCarol := delivered[carol]
		mu.Unlock()
		if len(toCarol) != 1 {
			t.Fatalf("carol got %d messages", len(toCarol))
		}
		if _, m, err := carolPhone.Receive(toCarol[0]); err != nil || m.GetConversation() != "Dune it is" {
			t.Fatalf("carol read %v: %v", m, err)
		}
		if status, _ := c.call("whatsapp_status", nil); strings.Contains(status.Detail, "Saving messages failed") {
			t.Fatalf("status = %+v", status)
		}
	})
}

func TestAddressBookFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(41)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		mama := node.JID{User: "40744444444", Server: node.ServerUser}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		mamaPhone, err := fakedevice.New(rand.Reader, mama)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range []*fakedevice.Device{ourPhone, mamaPhone} {
			if err := d.Upload(keys, 3); err != nil {
				t.Fatal(err)
			}
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(mama, fakeusync.Device{ID: 0})
		var mu sync.Mutex
		files := map[string][]byte{}
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if file, ok := files[req.URL.Path]; ok {
				_, _ = rw.Write(file)
				return
			}
			http.NotFound(rw, req)
		}))
		host := func(name string, plain []byte) (*wire.ExternalBlobReference, error) {
			mediaKey := bytes.Repeat([]byte{byte(len(name))}, media.KeySize)
			sealed, err := media.Encrypt(mediaKey, media.AppState, plain)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			defer mu.Unlock()
			path := "/v/appstate/" + name + ".enc"
			files[path] = sealed.File
			return &wire.ExternalBlobReference{MediaKey: mediaKey, DirectPath: new(path), FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:]}, nil
		}
		keyID, keyData := []byte{0, 0, 0, 2}, bytes.Repeat([]byte{5}, 32)
		book, err := fakeappstate.New(keyID, keyData, host)
		if err != nil {
			t.Fatal(err)
		}
		for number, name := range map[string][2]string{"40744444444": {"Mama Ioana", "Mama"}, "40755555555": {"Mamaia Hotel", "Mamaia"}} {
			value := &wire.SyncActionValue{ContactAction: &wire.SyncActionValue_ContactAction{FullName: new(name[0]), FirstName: new(name[1])}}
			if _, err := book.Patch(appstate.CriticalUnblockLow, fakeappstate.Set(value, "contact", number+"@s.whatsapp.net")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := book.Patch(appstate.RegularLow,
			fakeappstate.Set(&wire.SyncActionValue{Timestamp: new(int64(1790000000000)), PinAction: &wire.SyncActionValue_PinAction{Pinned: new(true)}}, "pin_v1", mama.String()),
			fakeappstate.Set(&wire.SyncActionValue{ArchiveChatAction: &wire.SyncActionValue_ArchiveChatAction{Archived: new(true)}}, "archive", "40755555555@s.whatsapp.net"),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := book.Patch(appstate.RegularHigh, fakeappstate.Set(&wire.SyncActionValue{MuteAction: &wire.SyncActionValue_MuteAction{Muted: new(true), MuteEndTimestamp: new(int64(-1))}}, "mute", mama.String())); err != nil {
			t.Fatal(err)
		}
		var delivered []node.Node
		inbox := make(chan node.Node)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, AppState: book,
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				if device == mama {
					delivered = append(delivered, stanza)
				}
			},
		}))
		c, _ := connect(t, w, cdn.Client())
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		if unknown, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "mama", "text": "hi"}); unknown.State != "unknown_recipient" {
			t.Fatalf("before the address book arrives: %+v", unknown)
		}
		share, err := ourPhone.SendPeer(keys, w.Phone.JID, &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
			Type: wire.Message_ProtocolMessage_APP_STATE_SYNC_KEY_SHARE.Enum(),
			AppStateSyncKeyShare: &wire.Message_AppStateSyncKeyShare{Keys: []*wire.Message_AppStateSyncKey{{
				KeyId: &wire.Message_AppStateSyncKeyId{KeyId: keyID}, KeyData: &wire.Message_AppStateSyncKeyData{KeyData: keyData},
			}}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		inbox <- fakerelay.DeliverPeer(ourPhone.JID, time.Now(), share)
		synctest.Wait()
		if ambiguous, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "mam", "text": "hi"}); ambiguous.State != "ambiguous_recipient" ||
			!strings.Contains(ambiguous.Detail, "Mama Ioana (+40744444444)") || !strings.Contains(ambiguous.Detail, "Mamaia Hotel (+40755555555)") {
			t.Fatalf("a partial name matching two contacts = %+v", ambiguous)
		}
		sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "MAMA", "text": "Pup, ajung diseara"})
		if sent.State != "sent" || sent.To != "Mama Ioana (+40744444444)" {
			t.Fatalf("send to mama = %+v", sent)
		}
		mu.Lock()
		got := slices.Clone(delivered)
		mu.Unlock()
		if len(got) != 1 {
			t.Fatalf("mama got %d messages", len(got))
		}
		if _, m, err := mamaPhone.Receive(got[0]); err != nil || m.GetConversation() != "Pup, ajung diseara" {
			t.Fatalf("mama read %v: %v", m, err)
		}
		chat, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "mama ioana"})
		if len(chat.Messages) != 1 || !chat.Messages[0].FromMe || chat.Messages[0].Chat != "Mama Ioana (+40744444444)" {
			t.Fatalf("the chat with mama = %+v", chat)
		}
		chats, result := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil)
		if len(chats.Chats) != 2 || !chats.Chats[0].Pinned || !chats.Chats[0].Muted || chats.Chats[0].Archived || !chats.Chats[1].Archived || chats.Chats[1].Pinned ||
			chats.Chats[1].LastActive != "" || fromText[mcptools.ChatsReport](t, result).Chats[1].Name != "Mamaia Hotel (+40755555555)" {
			t.Fatalf("chats = %+v\n%s", chats.Chats, textOf(result))
		}
	})
}

func TestSendingFilesFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(43)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 5); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		family := groups.Group{JID: node.JID{User: "120363000000000051", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
			Participants: []groups.Participant{{JID: account}, {JID: bob}}}
		var mu sync.Mutex
		var toBob []node.Node
		inbox := make(chan node.Node)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Groups: fakegroups.New(family),
			Members: func(node.JID) []node.JID { return []node.JID{bob, w.Phone.JID} },
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				if device == bob {
					toBob = append(toBob, stanza)
				}
			},
		}))
		cdn := fakecdn.New()
		c, _ := connect(t, w, httptest.NewTestServer(t, cdn).Client())
		if report, _ := callAs[mcptools.SendReport](c, "send_whatsapp_file", map[string]any{"to": "+40 722 222 222", "path": "/nope"}); report.State != "not_linked" {
			t.Fatalf("before linking: %+v", report)
		}
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()

		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		img := image.NewRGBA(image.Rect(0, 0, 320, 160))
		for x := range 320 {
			img.Set(x, x/2, color.RGBA{R: 255, A: 255})
		}
		var picture bytes.Buffer
		if err := png.Encode(&picture, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "sunset.png"), picture.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		report := []byte("%PDF-1.7 quarterly numbers")
		if err := os.WriteFile(filepath.Join(home, "Q3 report.pdf"), report, 0o600); err != nil {
			t.Fatal(err)
		}
		receive := func(t *testing.T) *wire.Message {
			t.Helper()
			mu.Lock()
			last := toBob[len(toBob)-1]
			mu.Unlock()
			_, m, err := bobPhone.Receive(last)
			if err != nil {
				t.Fatal(err)
			}
			return m
		}
		download := func(t *testing.T, m *wire.Message) []byte {
			t.Helper()
			ref, ok := media.ReferenceOf(m)
			if !ok {
				t.Fatalf("no media in %v", m)
			}
			file, ok := cdn.File(strings.SplitN(ref.DirectPath, "?", 2)[0])
			if !ok {
				t.Fatalf("the cdn has nothing at %s", ref.DirectPath)
			}
			plain, err := media.Decrypt(ref.MediaKey, ref.Type, file, ref.FileEncSHA256, ref.FileSHA256)
			if err != nil {
				t.Fatal(err)
			}
			return plain
		}

		sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_file", map[string]any{"to": "+40 722 222 222", "path": "~/sunset.png", "caption": " the view "})
		if sent.State != "sent" || sent.To != "+40722222222" || !strings.Contains(sent.Detail, "sunset.png (image,") {
			t.Fatalf("send photo = %+v", sent)
		}
		photo := receive(t).GetImageMessage()
		if photo.GetCaption() != "the view" || photo.GetWidth() != 320 || photo.GetHeight() != 160 || photo.GetMimetype() != "image/png" || len(photo.GetJpegThumbnail()) == 0 ||
			!bytes.Equal(download(t, &wire.Message{ImageMessage: photo}), picture.Bytes()) {
			t.Fatalf("bob got %v", photo)
		}
		reopened, result := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": sent.ID})
		if reopened.State != "ok" || reopened.Type != "image" || len(result.Content) != 2 {
			t.Fatalf("reopening a sent photo = %+v", reopened)
		}

		sent, _ = callAs[mcptools.SendReport](c, "send_whatsapp_file", map[string]any{"to": "family", "path": filepath.Join(home, "Q3 report.pdf"), "caption": "numbers"})
		if sent.State != "sent" || sent.To != "Family" {
			t.Fatalf("send document to the group = %+v", sent)
		}
		doc := media.Unwrap(receive(t))
		if doc.GetDocumentMessage().GetFileName() != "Q3 report.pdf" || doc.GetDocumentMessage().GetMimetype() != "application/pdf" || !bytes.Equal(download(t, doc), report) {
			t.Fatalf("the family got %v", doc)
		}

		for _, tt := range []struct {
			args  map[string]any
			state string
		}{
			{map[string]any{"to": "bob", "path": filepath.Join(home, "missing.txt")}, "file_not_found"},
			{map[string]any{"to": "bob", "path": home}, "not_a_file"},
			{map[string]any{"to": "bob", "path": "  "}, "file_not_found"},
			{map[string]any{"to": "nobody at all", "path": "~/sunset.png"}, "unknown_recipient"},
		} {
			if got, _ := callAs[mcptools.SendReport](c, "send_whatsapp_file", tt.args); got.State != tt.state {
				t.Fatalf("send_whatsapp_file(%v) = %+v", tt.args, got)
			}
		}
	})
}

func TestMessageLifecycleFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(47)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 5); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		cdn := fakecdn.New()
		photo := bytes.Repeat([]byte("old photo "), 300)
		mediaKey := bytes.Repeat([]byte{6}, media.KeySize)
		sealed, err := media.Encrypt(mediaKey, media.Image, photo)
		if err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		var sent []node.Node
		inbox := make(chan node.Node, 8)
		server := &fakeworld.Server{Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {}}
		server.Received = func(n node.Node) {
			mu.Lock()
			sent = append(sent, n)
			mu.Unlock()
			if kind, _ := n.Attr("type").Text(); n.Tag != "receipt" || kind != "server-error" {
				return
			}
			id, _ := n.Attr("id").Text()
			rmr, _ := n.Child("rmr")
			cdn.Put("/v/t62.7118-24/again.enc", sealed.File)
			ct, iv, err := mediaretry.Seal(rand.Reader, mediaKey, id, &wire.MediaRetryNotification{StanzaId: new(id), DirectPath: new("/v/t62.7118-24/again.enc"), Result: wire.MediaRetryNotification_SUCCESS.Enum()})
			if err != nil {
				t.Error(err)
			}
			go func() {
				inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("mediaretry")}, {Key: "id", Value: node.Text(id)}},
					Children: []node.Node{rmr, {Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: ct}, {Tag: "enc_iv", Bytes: iv}}}}}
			}()
		}
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(server))
		c, _ := connect(t, w, httptest.NewTestServer(t, cdn).Client())
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()

		ours, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 722 222 222", "text": "hi Bob"})
		from := func(m *wire.Message) string {
			t.Helper()
			out, err := bobPhone.Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			stanza := fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
			inbox <- stanza
			id, _ := out.Attr("id").Text()
			return id
		}
		protocol := func(target string, kind wire.Message_ProtocolMessage_Type, edited *wire.Message) *wire.Message {
			return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: &wire.MessageKey{RemoteJid: new(account.String()), FromMe: new(true), Id: new(target)}, Type: kind.Enum(), EditedMessage: edited}}
		}
		typo := from(&wire.Message{Conversation: new("helo")})
		from(protocol(typo, wire.Message_ProtocolMessage_MESSAGE_EDIT, &wire.Message{Conversation: new("hello")}))
		oops := from(&wire.Message{Conversation: new("wrong chat, sorry")})
		from(protocol(oops, wire.Message_ProtocolMessage_REVOKE, nil))
		from(&wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Key: &wire.MessageKey{RemoteJid: new(bob.String()), FromMe: new(false), Id: new(ours.ID)}, Text: new("👍")}})
		expired := from(&wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: new("/v/t62.7118-24/expired.enc"), MediaKey: mediaKey,
			FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:], Mimetype: new("image/png")}})
		synctest.Wait()

		read, result := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob"})
		if len(read.Messages) != 4 {
			t.Fatalf("read = %+v", read.Messages)
		}
		if marked, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": typo, "mark_read": true}); marked.State != "done" || !strings.Contains(marked.Detail, "as read") {
			t.Fatalf("mark_read = %+v", marked)
		}
		mine, edited, deleted := read.Messages[0], read.Messages[1], read.Messages[2]
		if mine.Text != "hi Bob" || !slices.Equal(mine.Reactions, []string{"👍 Bob"}) || edited.Text != "hello" || !edited.Edited || mine.Edited ||
			deleted.Text != "" || deleted.Kind != "deleted" || !fromText[mcptools.ReadReport](t, result).Messages[1].Edited ||
			!slices.Equal(fromText[mcptools.ReadReport](t, result).Messages[0].Reactions, []string{"👍 Bob"}) {
			t.Fatalf("read = %+v\n%s", read, textOf(result))
		}
		synctest.Wait()
		mu.Lock()
		var readIDs []string
		for _, n := range sent {
			if kind, _ := n.Attr("type").Text(); n.Tag == "receipt" && kind == "read" {
				first, _ := n.Attr("id").Text()
				readIDs = append(readIDs, first)
				if list, ok := n.Child("list"); ok {
					for _, item := range list.Children {
						id, _ := item.Attr("id").Text()
						readIDs = append(readIDs, id)
					}
				}
			}
		}
		mu.Unlock()
		if slices.Sort(readIDs); !slices.Equal(readIDs, slices.Sorted(slices.Values([]string{typo, expired}))) {
			t.Fatalf("read receipts for %v, want Bob's live messages %v", readIDs, []string{typo, expired})
		}
		opened, openedResult := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": expired})
		if opened.State != "ok" || len(openedResult.Content) != 2 {
			t.Fatalf("an expired photo = %+v", opened)
		}
		if again, _ := callAs[mcptools.MediaReport](c, "get_whatsapp_media", map[string]any{"message_id": expired}); again.State != "ok" {
			t.Fatalf("the renewed path was not kept: %+v", again)
		}
		mu.Lock()
		asked := 0
		for _, n := range sent {
			if kind, _ := n.Attr("type").Text(); n.Tag == "receipt" && kind == "server-error" {
				asked++
			}
		}
		mu.Unlock()
		if asked != 1 {
			t.Fatalf("the phone was asked %d times; the second open must use the saved path", asked)
		}
	})
}

func TestRepliesFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(53)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		carol := node.JID{User: "40733333333", Server: node.ServerUser}
		carolPhone, err := fakedevice.New(rand.Reader, carol)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range []*fakedevice.Device{bobPhone, carolPhone} {
			if err := d.Upload(keys, 5); err != nil {
				t.Fatal(err)
			}
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		devices.Set(carol, fakeusync.Device{ID: 0})
		var mu sync.Mutex
		var toBob, toCarol []node.Node
		inbox := make(chan node.Node, 8)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox,
			Deliver: func(device node.JID, stanza node.Node) {
				mu.Lock()
				defer mu.Unlock()
				switch device {
				case bob:
					toBob = append(toBob, stanza)
				case carol:
					toCarol = append(toCarol, stanza)
				}
			},
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		if unknown, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": "3EB0NOPE", "mark_read": true}); unknown.State != "unknown_message" {
			t.Fatalf("mark_read on an unknown message = %+v", unknown)
		}
		from := func(m *wire.Message) string {
			t.Helper()
			out, err := bobPhone.Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			stanza := fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
			inbox <- stanza
			id, _ := out.Attr("id").Text()
			return id
		}
		question := from(&wire.Message{Conversation: new("are you coming?")})
		regret := from(&wire.Message{Conversation: new("ignore this")})
		from(&wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: &wire.MessageKey{Id: new(regret)}, Type: wire.Message_ProtocolMessage_REVOKE.Enum()}})
		synctest.Wait()

		sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"reply_to": question, "text": "yes, 10 minutes"})
		if sent.State != "sent" || sent.To != "Bob (+40722222222)" || !strings.Contains(sent.Detail, "Replied in Bob") {
			t.Fatalf("reply = %+v", sent)
		}
		mu.Lock()
		last := toBob[len(toBob)-1]
		mu.Unlock()
		_, got, err := bobPhone.Receive(last)
		quote := got.GetExtendedTextMessage().GetContextInfo()
		if err != nil || got.GetExtendedTextMessage().GetText() != "yes, 10 minutes" || quote.GetStanzaId() != question || quote.GetParticipant() != bob.String() ||
			quote.GetQuotedMessage().GetConversation() != "are you coming?" || quote.RemoteJid != nil {
			t.Fatalf("bob got %v: %v", got, err)
		}
		private, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 733 333 333", "reply_to": question, "text": "Bob asks if you are coming"})
		if private.State != "sent" || private.To != "+40733333333" {
			t.Fatalf("a reply in another chat = %+v", private)
		}
		mu.Lock()
		forCarol := toCarol[len(toCarol)-1]
		mu.Unlock()
		if _, got, err := carolPhone.Receive(forCarol); err != nil || got.GetExtendedTextMessage().GetContextInfo().GetRemoteJid() != bob.String() || got.GetExtendedTextMessage().GetContextInfo().GetStanzaId() != question {
			t.Fatalf("carol got %v: %v", got, err)
		}
		read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob"})
		if len(read.Messages) != 3 {
			t.Fatalf("read = %+v", read.Messages)
		}
		mine := read.Messages[2]
		if !mine.FromMe || mine.ReplyTo != question || mine.Text != "yes, 10 minutes" || mine.Quote != `Bob: "are you coming?"` {
			t.Fatalf("our reply reads as %+v", mine)
		}
		withCarol, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "+40 733 333 333"})
		if len(withCarol.Messages) != 1 || withCarol.Messages[0].ReplyTo != question || withCarol.Messages[0].Quote != `Bob: "are you coming?"` || withCarol.Messages[0].Text != "Bob asks if you are coming" {
			t.Fatalf("the reply in carol's chat = %+v", withCarol.Messages)
		}
		for _, tt := range []struct {
			args  map[string]any
			state string
		}{
			{map[string]any{"reply_to": "3EB0NOSUCH", "text": "hi"}, "unknown_message"},
			{map[string]any{"reply_to": regret, "text": "hi"}, "deleted_message"},
			{map[string]any{"to": "+40 722 222 222", "reply_to": "3EB0NOSUCH", "text": "hi"}, "unknown_message"},
			{map[string]any{"to": "+40 722 222 222", "reply_to": regret, "text": "hi"}, "deleted_message"},
		} {
			if got, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", tt.args); got.State != tt.state {
				t.Fatalf("send %v = %+v", tt.args, got)
			}
		}
	})
}

func TestUnreadAndTicksFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(59)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 5); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		inbox := make(chan node.Node, 8)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {}}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		from := func(text string) {
			t.Helper()
			out, err := bobPhone.Send(keys, devices, account, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
		}
		unread := func() (int, string) {
			t.Helper()
			chats, result := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil)
			if len(chats.Chats) != 1 {
				t.Fatalf("chats = %+v", chats)
			}
			return chats.Chats[0].Unread, textOf(result)
		}
		from("are you there?")
		from("hello?")
		synctest.Wait()
		if count, listing := unread(); count != 2 || !strings.Contains(listing, `"unread":2`) {
			t.Fatalf("unread = %d\n%s", count, listing)
		}
		sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "bob", "text": "yes!"})
		receipt := func(kind, id string) node.Node {
			attrs := []node.Attr{{Key: "from", Value: node.Address(bob)}, {Key: "id", Value: node.Text(id)}, {Key: "t", Value: node.Text("1790000000")}}
			if kind != "" {
				attrs = append(attrs, node.Attr{Key: "type", Value: node.Text(kind)})
			}
			return node.Node{Tag: "receipt", Attrs: attrs}
		}
		inbox <- receipt("", sent.ID)
		synctest.Wait()
		if read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob", "limit": 1}); read.Messages[0].Status != "delivered" || read.Messages[0].Text != "yes!" {
			t.Fatalf("after the delivery receipt: %+v", read.Messages)
		}
		inbox <- receipt("read", sent.ID)
		synctest.Wait()
		missed, missedText := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"unread": true})
		if len(missed.Messages) != 2 || missed.Messages[0].Text != "are you there?" || missed.Messages[1].Text != "hello?" || missed.Detail != "2 unread message(s)." ||
			fromText[mcptools.ReadReport](t, missedText).Messages[1].ID != missed.Messages[1].ID {
			t.Fatalf("what did I miss = %+v", missed)
		}
		if mixed, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"unread": true, "query": "hello"}); mixed.State != "invalid_unread" {
			t.Fatalf("unread with a query = %+v", mixed)
		}
		if elsewhere, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"unread": true, "chat": "me"}); len(elsewhere.Messages) != 0 || elsewhere.Detail != "No unread messages." {
			t.Fatalf("unread in another chat = %+v", elsewhere)
		}
		read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob"})
		if len(read.Messages) != 3 || read.Messages[2].Status != "read" || read.Messages[2].Text != "yes!" || read.Messages[0].Status != "" ||
			!strings.Contains(read.Detail, "The phone may keep older ones: call again with before="+read.Messages[0].ID) {
			t.Fatalf("after the read receipt: %+v", read)
		}
		if marked, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": read.Messages[0].ID, "mark_read": true}); marked.State != "done" || !strings.Contains(marked.Detail, "as read") {
			t.Fatalf("mark_read = %+v", marked)
		}
		if count, _ := unread(); count != 0 {
			t.Fatalf("unread after marking read = %d", count)
		}
		from("one more")
		synctest.Wait()
		if count, _ := unread(); count != 1 {
			t.Fatalf("unread after a new message = %d", count)
		}
		missed, _ = callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"unread": true, "chat": "bob"})
		if len(missed.Messages) != 1 || missed.Messages[0].Text != "one more" {
			t.Fatalf("the one new message = %+v", missed)
		}
		if marked, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": missed.Messages[0].ID, "mark_read": true}); marked.Detail != "Marked 1 message(s) in Bob (+40722222222) as read; the senders see blue ticks." {
			t.Fatalf("mark_read = %+v", marked)
		}
		if again, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": missed.Messages[0].ID, "mark_read": true}); again.State != "done" || !strings.HasPrefix(again.Detail, "Nothing in ") {
			t.Fatalf("mark_read with nothing unread = %+v", again)
		}
		if count, _ := unread(); count != 0 {
			t.Fatalf("unread after reading the new message = %d", count)
		}
		from("and another")
		synctest.Wait()
		inbox <- receipt("read-self", "3EB0SELF")
		synctest.Wait()
		if count, listing := unread(); count != 0 || strings.Contains(listing, "unread") {
			t.Fatalf("unread after reading on the phone = %d\n%s", count, listing)
		}
	})
}

func TestPollsFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(53)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range []*fakedevice.Device{ourPhone, bobPhone} {
			if err := d.Upload(keys, 5); err != nil {
				t.Fatal(err)
			}
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		inbox := make(chan node.Node, 8)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {}}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()

		push := func(from *fakedevice.Device, to node.JID, m *wire.Message) string {
			t.Helper()
			out, err := from.Send(keys, devices, to, m)
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(from.JID, map[node.JID]string{bob: "Bob", account: "Me"}[from.JID], time.Now(), out)[w.Phone.JID]
			id, _ := out.Attr("id").Text()
			return id
		}
		secret := bytes.Repeat([]byte{4}, 32)
		pollID := push(bobPhone, account, &wire.Message{
			PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), SelectableOptionsCount: new(uint32(1)), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}}},
			MessageContextInfo:    &wire.MessageContextInfo{MessageSecret: secret},
		})
		synctest.Wait()
		vote := func(from *fakedevice.Device, to, voter node.JID, pollID string, ms int64, options ...string) {
			t.Helper()
			enc, err := message.SealVote(rand.Reader, message.Ballot{Secret: secret, PollID: pollID, Creator: bob, Voter: voter}, options)
			if err != nil {
				t.Fatal(err)
			}
			push(from, to, &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{
				PollCreationMessageKey: &wire.MessageKey{RemoteJid: new(bob.String()), FromMe: new(false), Id: new(pollID)},
				Vote:                   enc, SenderTimestampMs: new(ms),
			}})
		}
		now := time.Now().UnixMilli()
		vote(bobPhone, account, bob, pollID, now+1, "Sushi")
		vote(ourPhone, bob, account, pollID, now+2, "Pizza")
		vote(bobPhone, account, bob, pollID, now+3, "Pizza")
		vote(bobPhone, account, bob, "3EB0NOPOLL", now+4, "Sushi")
		vote(bobPhone, account, account, pollID, now+5, "Sushi")
		vote(bobPhone, account, bob, pollID, now+6, "Pizza", "Sushi")
		synctest.Wait()

		read, result := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob"})
		if len(read.Messages) != 1 {
			t.Fatalf("votes must not read as messages: %+v", read.Messages)
		}
		if poll := read.Messages[0]; poll.Kind != "poll" || poll.Text != "[poll] Lunch? Options: Pizza (2: me, Bob) / Sushi (0)" || !strings.Contains(textOf(result), "Pizza (2: me, Bob)") {
			t.Fatalf("poll = %+v", poll)
		}
		chats, _ := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil)
		if len(chats.Chats) != 1 || chats.Chats[0].Unread != 1 {
			t.Fatalf("votes must not count as unread messages: %+v", chats.Chats)
		}
	})
}

func TestChangingMessagesFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(59)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 9); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		var mu sync.Mutex
		var toBob []node.Node
		inbox := make(chan node.Node, 8)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox,
			Deliver: func(device node.JID, stanza node.Node) {
				if device.User == bob.User {
					mu.Lock()
					toBob = append(toBob, stanza)
					mu.Unlock()
				}
			},
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		push := func(m *wire.Message) string {
			t.Helper()
			out, err := bobPhone.Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
			id, _ := out.Attr("id").Text()
			return id
		}
		bobGot := func() (message.Incoming, *wire.Message) {
			t.Helper()
			mu.Lock()
			last := toBob[len(toBob)-1]
			mu.Unlock()
			in, m, err := bobPhone.Receive(last)
			if err != nil {
				t.Fatal(err)
			}
			return in, m
		}
		mine, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 722 222 222", "text": "helo"})
		theirs := push(&wire.Message{Conversation: new("are you there?")})
		secret := bytes.Repeat([]byte{3}, 32)
		poll := push(&wire.Message{
			PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), SelectableOptionsCount: new(uint32(1)), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}, {OptionName: new("Sushi")}}},
			MessageContextInfo:    &wire.MessageContextInfo{MessageSecret: secret},
		})
		synctest.Wait()

		for _, tt := range []struct {
			args   map[string]any
			state  string
			detail string
		}{
			{args: map[string]any{"message_id": theirs}, state: "choose_one"},
			{args: map[string]any{"message_id": theirs, "react": "👍", "delete": true}, state: "choose_one"},
			{args: map[string]any{"message_id": "3EB0NOTHING", "react": "👍"}, state: "unknown_message"},
			{args: map[string]any{"message_id": theirs, "edit": "hacked"}, state: "not_yours"},
			{args: map[string]any{"message_id": theirs, "delete": true}, state: "not_yours"},
			{args: map[string]any{"message_id": mine.ID, "edit": "  "}, state: "empty_message"},
			{args: map[string]any{"message_id": mine.ID, "edit": ""}, state: "empty_message", detail: "The new text is empty"},
			{args: map[string]any{"message_id": theirs, "react": "not an emoji"}, state: "not_an_emoji", detail: "single emoji"},
			{args: map[string]any{"message_id": theirs, "react": ""}, state: "not_an_emoji"},
			{args: map[string]any{"message_id": theirs, "react": "👍👍"}, state: "not_an_emoji"},
			{args: map[string]any{"message_id": theirs, "vote": []string{"Pizza"}}, state: "not_a_poll"},
			{args: map[string]any{"message_id": poll, "vote": []string{"Tacos"}}, state: "invalid_vote", detail: "The options are: Pizza / Sushi."},
			{args: map[string]any{"message_id": poll, "vote": []string{"Pizza", "Sushi"}}, state: "invalid_vote", detail: "This poll allows at most 1 choice(s). The options are"},
			{args: map[string]any{"message_id": poll, "vote": []string{"Tacos"}}, state: "invalid_vote", detail: `"Tacos" is not one of its options. The options are: Pizza / Sushi.`},
			{args: map[string]any{"message_id": poll, "vote": []string{}}, state: "choose_one", detail: "remove_vote"},
			{args: map[string]any{"message_id": theirs, "remove_vote": true}, state: "not_a_poll"},
		} {
			if report, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", tt.args); report.State != tt.state || !strings.Contains(report.Detail, tt.detail) {
				t.Fatalf("change %v = %+v, want %s", tt.args, report, tt.state)
			}
		}

		reacted, result := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": theirs, "react": "👍"})
		in, m := bobGot()
		if reacted.State != "done" || reacted.Detail != `Reacted 👍 to "are you there?" in Bob (+40722222222).` || fromText[mcptools.ChangeReport](t, result) != reacted ||
			m.GetReactionMessage().GetText() != "👍" || m.GetReactionMessage().GetKey().GetId() != theirs || m.GetReactionMessage().GetKey().GetFromMe() || in.Edit != message.EditNone {
			t.Fatalf("reaction = %+v; Bob got %v (edit %d)", reacted, m, in.Edit)
		}
		callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": theirs, "remove_reaction": true})
		if in, m := bobGot(); m.GetReactionMessage().GetText() != "" || m.GetReactionMessage() == nil || in.Edit != message.EditSenderRevoke {
			t.Fatalf("taking the reaction back: Bob got %v (edit %d)", m, in.Edit)
		}
		edited, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": mine.ID, "edit": "hello"})
		if in, m := bobGot(); edited.State != "done" || m.GetProtocolMessage().GetType() != wire.Message_ProtocolMessage_MESSAGE_EDIT ||
			m.GetProtocolMessage().GetEditedMessage().GetConversation() != "hello" || m.GetProtocolMessage().GetKey().GetId() != mine.ID || in.Edit != message.EditMessage {
			t.Fatalf("edit = %+v; Bob got %v (edit %d)", edited, m, in.Edit)
		}
		voted, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": poll, "vote": []string{" pizza "}})
		_, m = bobGot()
		opened, err := message.OpenVote(message.Ballot{Secret: secret, PollID: poll, Creator: bob, Voter: account}, m.GetPollUpdateMessage().GetVote())
		if voted.State != "done" || voted.Detail != `Voted Pizza in "Lunch?" in Bob (+40722222222).` || err != nil || len(opened.GetSelectedOptions()) != 1 || !bytes.Equal(opened.GetSelectedOptions()[0], message.OptionHash("Pizza")) {
			t.Fatalf("vote = %+v; Bob opened %v, %v", voted, opened, err)
		}
		mu.Lock()
		meta, _ := toBob[len(toBob)-1].Child("meta")
		mu.Unlock()
		if meta.Attr("polltype").String() != "vote" {
			t.Fatalf("the vote stanza carries meta %s", meta)
		}
		synctest.Wait()
		read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob"})
		if len(read.Messages) != 3 || read.Messages[0].Text != "hello" || !read.Messages[0].Edited || read.Messages[1].Text != "are you there?" ||
			read.Messages[2].Text != "[poll] Lunch? Options: Pizza (1: me) / Sushi (0)" {
			t.Fatalf("after the changes: %+v", read.Messages)
		}
		deleted, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": mine.ID, "delete": true})
		if in, m := bobGot(); deleted.State != "done" || m.GetProtocolMessage().GetType() != wire.Message_ProtocolMessage_REVOKE || m.GetProtocolMessage().GetKey().GetId() != mine.ID || in.Edit != message.EditSenderRevoke {
			t.Fatalf("delete = %+v; Bob got %v (edit %d)", deleted, m, in.Edit)
		}
		synctest.Wait()
		if again, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": mine.ID, "edit": "back"}); again.State != "deleted_message" {
			t.Fatalf("editing a deleted message = %+v", again)
		}
		late, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 722 222 222", "text": "see you"})
		time.Sleep(16 * time.Minute)
		if tooLate, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": late.ID, "edit": "see you soon"}); tooLate.State != "too_late" || !strings.Contains(tooLate.Detail, "16m") {
			t.Fatalf("a late edit = %+v", tooLate)
		}
		if stillOK, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": late.ID, "delete": true}); stillOK.State != "done" {
			t.Fatalf("a delete within the window = %+v", stillOK)
		}
		for _, bad := range []map[string]any{
			{"to": "bob", "poll": map[string]any{"question": "Lunch?", "options": []string{"Pizza"}}},
			{"to": "bob", "poll": map[string]any{"question": "Lunch?", "options": []string{"Pizza", " pizza "}}},
			{"to": "bob", "poll": map[string]any{"question": " ", "options": []string{"Pizza", "Sushi"}}},
			{"to": "bob", "poll": map[string]any{"question": strings.Repeat("q", 256), "options": []string{"Pizza", "Sushi"}}},
		} {
			if refused, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", bad); refused.State != "invalid_poll" || !strings.HasPrefix(refused.Detail, "Not sent: ") {
				t.Fatalf("send %v = %+v", bad, refused)
			}
		}
		if both, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "bob", "text": "hi", "poll": map[string]any{"question": "q", "options": []string{"a", "b"}}}); both.State != "choose_one" {
			t.Fatalf("text and poll together = %+v", both)
		}
		asked, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "bob", "poll": map[string]any{"question": "Dinner?", "options": []string{" Pasta ", "Soup", "Salad"}, "multiple_answers": true}})
		_, created := bobGot()
		mu.Lock()
		creationMeta, _ := toBob[len(toBob)-1].Child("meta")
		mu.Unlock()
		ours := created.GetPollCreationMessage()
		if asked.State != "sent" || asked.Detail != "Sent the poll to Bob (+40722222222)." || ours.GetName() != "Dinner?" || len(ours.GetOptions()) != 3 ||
			ours.GetOptions()[0].GetOptionName() != "Pasta" || ours.GetSelectableOptionsCount() != 0 || len(created.GetMessageContextInfo().GetMessageSecret()) != 32 ||
			creationMeta.Attr("polltype").String() != "creation" {
			t.Fatalf("poll = %+v; Bob got %v, meta %s", asked, created, creationMeta)
		}
		mine2, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": asked.ID, "vote": []string{"soup", "Salad"}})
		_, cast := bobGot()
		ourLID := node.JID{User: "987654321", Server: node.ServerLID}
		ballot := message.Ballot{Secret: created.GetMessageContextInfo().GetMessageSecret(), PollID: asked.ID, Creator: ourLID, Voter: ourLID}
		if opened, err := message.OpenVote(ballot, cast.GetPollUpdateMessage().GetVote()); mine2.State != "done" || err != nil || len(opened.GetSelectedOptions()) != 2 {
			t.Fatalf("voting on our own poll = %+v; Bob opened %v, %v", mine2, opened, err)
		}
		synctest.Wait()
		if read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob", "limit": 1}); read.Messages[0].Kind != "poll" ||
			!strings.Contains(read.Messages[0].Text, "Dinner? Options: Pasta (0) / Soup (1: me) / Salad (1: me)") {
			t.Fatalf("our poll reads as %+v", read.Messages)
		}
		withdrawn, _ := callAs[mcptools.ChangeReport](c, "change_whatsapp_message", map[string]any{"message_id": asked.ID, "remove_vote": true})
		_, empty := bobGot()
		if opened, err := message.OpenVote(ballot, empty.GetPollUpdateMessage().GetVote()); withdrawn.State != "done" || withdrawn.Detail != `Took back the vote in "Dinner?" in Bob (+40722222222).` ||
			err != nil || len(opened.GetSelectedOptions()) != 0 {
			t.Fatalf("taking the vote back = %+v; Bob opened %v, %v", withdrawn, opened, err)
		}
		synctest.Wait()
		if read, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "bob", "limit": 1}); !strings.Contains(read.Messages[0].Text, "Dinner? Options: Pasta / Soup / Salad") {
			t.Fatalf("after taking the vote back: %+v", read.Messages)
		}
	})
}

func TestStatusUpdatesFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(61)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		bobPhone, err := fakedevice.New(rand.Reader, bob)
		if err != nil {
			t.Fatal(err)
		}
		if err := bobPhone.Upload(keys, 5); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		devices.Set(bob, fakeusync.Device{ID: 0})
		inbox := make(chan node.Node, 4)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {}}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		push := func(m *wire.Message, status bool) {
			t.Helper()
			out, err := bobPhone.Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			stanza := fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
			if status {
				stanza.Attrs[0] = node.Attr{Key: "from", Value: node.Address(node.StatusBroadcast())}
				stanza.Attrs = append(stanza.Attrs, node.Attr{Key: "participant", Value: node.Address(bob)})
			}
			inbox <- stanza
		}
		push(&wire.Message{Conversation: new("hi")}, false)
		push(&wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("at the beach")}}, true)
		synctest.Wait()

		chats, _ := callAs[mcptools.ChatsReport](c, "list_whatsapp_chats", nil)
		if len(chats.Chats) != 1 || chats.Chats[0].Unread != 1 {
			t.Fatalf("chats = %+v", chats.Chats)
		}
		if all, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", nil); len(all.Messages) != 1 || all.Messages[0].Text != "hi" {
			t.Fatalf("all chats = %+v", all.Messages)
		}
		statuses, _ := callAs[mcptools.ReadReport](c, "read_whatsapp_messages", map[string]any{"chat": "Status"})
		if len(statuses.Messages) != 1 || statuses.Messages[0].Text != "at the beach" || statuses.Messages[0].Chat != "status updates" || statuses.Messages[0].From != "Bob (+40722222222)" {
			t.Fatalf("status updates = %+v", statuses.Messages)
		}
		if posted, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "status@broadcast", "text": "hello world"}); posted.State != "unsupported_recipient" {
			t.Fatalf("posting a status = %+v", posted)
		}
	})
}

func TestLinkingAgainAfterThePhoneRemovesTheDevice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(67)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		inbox := make(chan node.Node, 1)
		server := &fakeworld.Server{Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox, Deliver: func(node.JID, node.Node) {}}
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(server), w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(server))
		c, _ := connect(t, w)
		link := func() {
			t.Helper()
			linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
			w.Type(linking.Code)
			c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
			synctest.Wait()
			if status, _ := c.call("whatsapp_status", nil); status.State != "linked" || status.Connection != "connected" {
				t.Fatalf("status = %+v", status)
			}
		}
		link()
		inbox <- node.Node{Tag: "stream:error", Attrs: []node.Attr{{Key: "code", Value: node.Text("401")}}, Children: []node.Node{{Tag: "conflict", Attrs: []node.Attr{{Key: "type", Value: node.Text("device_removed")}}}}}
		synctest.Wait()
		status, result := c.call("whatsapp_status", nil)
		if status.State != "logged_out" || !strings.Contains(textOf(result), "link_whatsapp to link again") {
			t.Fatalf("after the phone removed the device: %+v", status)
		}
		if sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "me", "text": "hi"}); sent.State != "not_linked" {
			t.Fatalf("sending while logged out = %+v", sent)
		}
		dials := w.Dials()
		time.Sleep(5 * time.Minute)
		if w.Dials() != dials {
			t.Fatalf("it kept dialing after a logout: %d more", w.Dials()-dials)
		}
		link()
	})
}

func TestSendingIsPacedToProtectTheAccount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(71)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		ourPhone, err := fakedevice.New(rand.Reader, account)
		if err != nil {
			t.Fatal(err)
		}
		if err := ourPhone.Upload(keys, 3); err != nil {
			t.Fatal(err)
		}
		devices.Set(account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: make(chan node.Node), Deliver: func(node.JID, node.Node) {},
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		for i := range messenger.MaxPerMinute {
			if sent, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "me", "text": "note " + strconv.Itoa(i)}); sent.State != "sent" {
				t.Fatalf("message %d = %+v", i, sent)
			}
		}
		refused, result := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "me", "text": "one too many"})
		if refused.State != "slow_down" || !strings.Contains(textOf(result), "Wait a minute") {
			t.Fatalf("message %d = %+v", messenger.MaxPerMinute, refused)
		}
		time.Sleep(time.Minute + time.Second)
		if later, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "me", "text": "one too many"}); later.State != "sent" {
			t.Fatalf("a minute later = %+v", later)
		}
	})
}

func TestForwardingFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(73)
		if err != nil {
			t.Fatal(err)
		}
		keys, devices := fakekeys.New(), fakeusync.New()
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		carol := node.JID{User: "40733333333", Server: node.ServerUser}
		phones := map[node.JID]*fakedevice.Device{}
		for _, j := range []node.JID{bob, carol} {
			d, err := fakedevice.New(rand.Reader, j)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Upload(keys, 6); err != nil {
				t.Fatal(err)
			}
			phones[j] = d
			devices.Set(j, fakeusync.Device{ID: 0})
		}
		devices.Set(account, fakeusync.Device{ID: w.Phone.JID.Device})
		var mu sync.Mutex
		var toCarol []node.Node
		inbox := make(chan node.Node, 8)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: keys, Devices: devices, PushName: "Me", Inbox: inbox,
			Deliver: func(device node.JID, stanza node.Node) {
				if device == carol {
					mu.Lock()
					toCarol = append(toCarol, stanza)
					mu.Unlock()
				}
			},
		}))
		cdn := fakecdn.New()
		c, _ := connect(t, w, httptest.NewTestServer(t, cdn).Client())
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		fromBob := func(m *wire.Message) string {
			t.Helper()
			out, err := phones[bob].Send(keys, devices, account, m)
			if err != nil {
				t.Fatal(err)
			}
			inbox <- fakerelay.Deliver(bob, "Bob", time.Now(), out)[w.Phone.JID]
			id, _ := out.Attr("id").Text()
			return id
		}
		carolGot := func() *wire.Message {
			t.Helper()
			mu.Lock()
			last := toCarol[len(toCarol)-1]
			mu.Unlock()
			_, m, err := phones[carol].Receive(last)
			if err != nil {
				t.Fatal(err)
			}
			return m
		}
		voice := bytes.Repeat([]byte("OggS voice "), 200)
		mediaKey := bytes.Repeat([]byte{8}, media.KeySize)
		sealed, err := media.Encrypt(mediaKey, media.Voice, voice)
		if err != nil {
			t.Fatal(err)
		}
		cdn.Put("/v/t62.7117-24/voice.enc", sealed.File)
		text := fromBob(&wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("meet at 5"), ContextInfo: &wire.ContextInfo{StanzaId: new("3EB0OLD"), Participant: new(account.String())}}})
		note := fromBob(&wire.Message{AudioMessage: &wire.Message_AudioMessage{
			DirectPath: new("/v/t62.7117-24/voice.enc"), MediaKey: mediaKey, FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:],
			Mimetype: new("audio/ogg; codecs=opus"), Ptt: new(true), Seconds: new(uint32(3)),
		}})
		viral := fromBob(&wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("chain letter"), ContextInfo: &wire.ContextInfo{IsForwarded: new(true), ForwardingScore: new(uint32(4))}}})
		poll := fromBob(&wire.Message{PollCreationMessageV3: &wire.Message_PollCreationMessage{Name: new("Lunch?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("Pizza")}}}})
		once := fromBob(&wire.Message{ViewOnceMessageV2: &wire.Message_FutureProofMessage{Message: &wire.Message{ImageMessage: &wire.Message_ImageMessage{
			DirectPath: new("/v/once.enc"), MediaKey: mediaKey, FileSha256: []byte{1}, FileEncSha256: []byte{2}, ViewOnce: new(true),
		}}}})
		synctest.Wait()

		for _, tt := range []struct {
			args  map[string]any
			state string
		}{
			{map[string]any{"to": "+40 733 333 333", "forward": poll}, "not_forwardable"},
			{map[string]any{"to": "+40 733 333 333", "forward": once}, "not_forwardable"},
			{map[string]any{"to": "+40 733 333 333", "forward": "3EB0NOTHING"}, "unknown_message"},
			{map[string]any{"to": "+40 733 333 333", "forward": text, "text": "and this"}, "choose_one"},
		} {
			if got, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", tt.args); got.State != tt.state {
				t.Fatalf("send %v = %+v, want %s", tt.args, got, tt.state)
			}
		}

		sent, result := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 733 333 333", "forward": text})
		got := carolGot()
		if context := got.GetExtendedTextMessage().GetContextInfo(); sent.State != "sent" || !strings.Contains(textOf(result), "Sent the forwarded message to") ||
			got.GetExtendedTextMessage().GetText() != "meet at 5" || !context.GetIsForwarded() || context.GetForwardingScore() != 1 || context.GetStanzaId() != "" {
			t.Fatalf("forwarded text = %+v; carol got %v", sent, got)
		}
		callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 733 333 333", "forward": viral})
		if context := carolGot().GetExtendedTextMessage().GetContextInfo(); !context.GetIsForwarded() || context.GetForwardingScore() != 5 {
			t.Fatalf("a message forwarded four times before: %v", context)
		}
		if forwarded, _ := callAs[mcptools.SendReport](c, "send_whatsapp_message", map[string]any{"to": "+40 733 333 333", "forward": note}); forwarded.State != "sent" {
			t.Fatalf("forwarded voice note = %+v", forwarded)
		}
		audio := carolGot().GetAudioMessage()
		if !audio.GetPtt() || audio.GetSeconds() != 3 || audio.GetDirectPath() == "/v/t62.7117-24/voice.enc" || bytes.Equal(audio.GetMediaKey(), mediaKey) || !audio.GetContextInfo().GetIsForwarded() {
			t.Fatalf("carol's voice note = %v", audio)
		}
		file, ok := cdn.File(strings.SplitN(audio.GetDirectPath(), "?", 2)[0])
		if !ok {
			t.Fatalf("nothing uploaded at %s", audio.GetDirectPath())
		}
		if plain, err := media.Decrypt(audio.GetMediaKey(), media.Voice, file, audio.GetFileEncSha256(), audio.GetFileSha256()); err != nil || !bytes.Equal(plain, voice) {
			t.Fatalf("carol could not open the voice note: %v", err)
		}
	})
}

func TestTheQRPageWhenNoBrowserOpens(t *testing.T) {
	for _, tt := range []struct {
		name   string
		page   func(context.Context) (string, bool, error)
		detail string
		url    string
	}{
		{name: "no browser", page: func(context.Context) (string, bool, error) { return "http://127.0.0.1:2/ab/", false, nil },
			detail: "open this page in a browser on this computer", url: "http://127.0.0.1:2/ab/"},
		{name: "no page", page: func(context.Context) (string, bool, error) { return "", false, errors.New("no port") },
			detail: "Show the user this QR code."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, err := fakeworld.New(33)
				if err != nil {
					t.Fatal(err)
				}
				w.Script(w.QRPairing(15*time.Second), w.Login(fakeworld.Success()))
				c, _ := connectWith(t, w, mcptools.Options{LinkPage: tt.page})
				report, result := c.call("link_whatsapp", nil)
				images := 0
				for _, content := range result.Content {
					if _, ok := content.(*mcp.ImageContent); ok {
						images++
					}
				}
				if report.State != "waiting_for_scan" || !strings.Contains(report.Detail, tt.detail) || report.Page != tt.url || images != 1 {
					t.Fatalf("link_whatsapp = %+v, %d images", report, images)
				}
				c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
			})
		})
	}
}

var portableName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func unportable(schema any) string {
	switch v := schema.(type) {
	case map[string]any:
		if types, ok := v["type"].([]any); ok {
			return fmt.Sprintf("type %v", types)
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf", "$ref", "not"} {
			if _, ok := v[key]; ok {
				return key
			}
		}
		for _, child := range v {
			if problem := unportable(child); problem != "" {
				return problem
			}
		}
	case []any:
		for _, child := range v {
			if problem := unportable(child); problem != "" {
				return problem
			}
		}
	}
	return ""
}

func TestStatusMentionsANewerChatwire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(91)
		if err != nil {
			t.Fatal(err)
		}
		newer := true
		c, _ := connectWith(t, w, mcptools.Options{Update: func() (string, bool) { return "v9.9.9", newer }})
		report, _ := c.call("whatsapp_status", nil)
		if report.Update != "v9.9.9" || !strings.Contains(report.Detail, "chatwire update") {
			t.Fatalf("with a newer release out: %+v", report)
		}
		newer = false
		if report, _ := c.call("whatsapp_status", nil); report.Update != "" || strings.Contains(report.Detail, "chatwire update") {
			t.Fatalf("when up to date: %+v", report)
		}
	})
}

func TestManagingAGroupFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(37)
		if err != nil {
			t.Fatal(err)
		}
		account := node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		carol := node.JID{User: "40733333333", Server: node.ServerUser}
		shy := node.JID{User: "40744444444", Server: node.ServerUser}
		family := groups.Group{JID: node.JID{User: "120363000000000011", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
			Participants: []groups.Participant{{JID: account, Admin: true}, {JID: bob}}, Description: "Sunday lunch", DescriptionID: "D1"}
		work := groups.Group{JID: node.JID{User: "120363000000000012", Server: node.ServerGroup}, Subject: "Work", Created: time.Unix(1700000000, 0),
			Participants: []groups.Participant{{JID: account}, {JID: bob, Admin: true}}}
		groupsServer := fakegroups.New(family, work)
		groupsServer.Actor, groupsServer.InviteOnly = account, map[node.JID]bool{shy: true}
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{
			Keys: fakekeys.New(), Devices: fakeusync.New(), PushName: "Me", Inbox: make(chan node.Node), Groups: groupsServer,
		}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		manage := func(args map[string]any) mcptools.GroupReport {
			t.Helper()
			report, _ := callAs[mcptools.GroupReport](c, "manage_whatsapp_group", args)
			return report
		}
		if r := manage(map[string]any{"group": "family", "action": "rename", "text": "Family 2026"}); r.State != "done" {
			t.Fatalf("rename: %+v", r)
		}
		if r := manage(map[string]any{"group": "family 2026", "action": "describe", "text": "Lunch at 1"}); r.State != "done" {
			t.Fatalf("describe: %+v", r)
		}
		r := manage(map[string]any{"group": "family 2026", "action": "add", "people": []string{"+40 733 333 333", "+40744444444", "+40722222222"}})
		if r.State != "partly_done" || len(r.Changed) != 1 || len(r.Failed) != 2 || !strings.Contains(r.Failed[0].Reason, "invite") || r.Failed[1].Reason != "already in the group" {
			t.Fatalf("add: %+v", r)
		}
		if r := manage(map[string]any{"group": "family 2026", "action": "make_admin", "people": []string{"+40733333333"}}); r.State != "done" {
			t.Fatalf("make_admin: %+v", r)
		}
		if r := manage(map[string]any{"group": "family 2026", "action": "remove", "people": []string{"+40722222222"}}); r.State != "done" {
			t.Fatalf("remove: %+v", r)
		}
		after, _ := groupsServer.Group(family.JID)
		if after.Subject != "Family 2026" || after.Description != "Lunch at 1" || len(after.Participants) != 2 || after.Participants[1].JID != carol || !after.Participants[1].Admin {
			t.Fatalf("the group after the changes: %+v", after)
		}
		if r := manage(map[string]any{"group": "work", "action": "rename", "text": "Mine now"}); r.State != "not_admin" {
			t.Fatalf("renaming a group we do not run: %+v", r)
		}
		if r := manage(map[string]any{"group": "family 2026", "action": "rename", "text": strings.Repeat("x", 101)}); r.State != "invalid" {
			t.Fatalf("a name that is too long: %+v", r)
		}
		if r := manage(map[string]any{"group": "family 2026", "action": "dance"}); r.State != "unknown_action" {
			t.Fatalf("an unknown action: %+v", r)
		}
		if r := manage(map[string]any{"group": "work", "action": "leave"}); r.State != "done" {
			t.Fatalf("leave: %+v", r)
		}
		if left, _ := groupsServer.Group(work.JID); len(left.Participants) != 1 {
			t.Fatalf("still in work after leaving: %+v", left.Participants)
		}
	})
}

func TestCheckingNumbersFromClaude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(38)
		if err != nil {
			t.Fatal(err)
		}
		bob := node.JID{User: "40722222222", Server: node.ServerUser}
		devices := fakeusync.New()
		devices.Set(bob, fakeusync.Device{ID: 0})
		devices.Profile(bob, node.JID{User: "99001", Server: node.ServerLID}, "At the gym")
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()), w.Serve(&fakeworld.Server{Keys: fakekeys.New(), Devices: devices, PushName: "Me", Inbox: make(chan node.Node)}))
		c, _ := connect(t, w)
		linking, _ := c.call("link_whatsapp", map[string]any{"phone_number": "+40 700 000 000"})
		w.Type(linking.Code)
		c.call("whatsapp_status", map[string]any{"wait_seconds": 60})
		synctest.Wait()
		report, result := callAs[mcptools.LookUpReport](c, "check_whatsapp_numbers", map[string]any{"numbers": []string{"+40 722 222 222", "+40799999999"}})
		if report.State != "ok" || len(report.Numbers) != 2 || !report.Numbers[0].OnWhatsApp || report.Numbers[0].About != "At the gym" ||
			report.Numbers[1].OnWhatsApp || report.Numbers[1].Number != "+40799999999" || !strings.Contains(textOf(result), "1 of 2 on WhatsApp") {
			t.Fatalf("lookup = %+v\n%s", report, textOf(result))
		}
		many := make([]string, 11)
		for i := range many {
			many[i] = fmt.Sprintf("+4072000000%d", i)
		}
		if r, _ := callAs[mcptools.LookUpReport](c, "check_whatsapp_numbers", map[string]any{"numbers": many}); r.State != "too_many" {
			t.Fatalf("eleven numbers: %+v", r)
		}
		if r, _ := callAs[mcptools.LookUpReport](c, "check_whatsapp_numbers", map[string]any{"numbers": []string{"bob"}}); r.State != "invalid_number" {
			t.Fatalf("a name instead of a number: %+v", r)
		}
	})
}
