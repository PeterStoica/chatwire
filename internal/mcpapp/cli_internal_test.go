package mcpapp

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/mcptools"
	"github.com/PeterStoica/chatwire/internal/setup"
)

func TestReportsReadAsCommands(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ detail, want string }{
		{"The linking code or QR code expired before it was used. Call link_whatsapp again for a fresh one.", "Run chatwire link again for a fresh one."},
		{"Linking failed: the phone said no. Call link_whatsapp to try again.", "Linking failed: the phone said no. Run chatwire link to try again."},
		{"WhatsApp is not linked yet. Call link_whatsapp to show the user a QR code to scan.", "WhatsApp is not linked yet. Run chatwire link to get a QR code to scan."},
		{"WhatsApp unlinked this computer. Call link_whatsapp to link again.", "WhatsApp unlinked this computer. Run chatwire link to link again."},
		{"It changes every 20 seconds; if it stops working, call link_whatsapp again, or link with the phone number instead.", "if it stops working, run chatwire link again"},
		{"The page keeps up as the code changes and says when linking is done. Tell the user, then call whatsapp_status with wait_seconds.", "Tell the user, then run chatwire status --wait 50 until it is linked."},
	} {
		got := asCommands.Replace(tt.detail)
		if !strings.Contains(got, tt.want) || strings.Contains(got, "link_whatsapp") || strings.Contains(got, "whatsapp_status") || strings.Contains(got, "wait_seconds") {
			t.Errorf("%q became %q", tt.detail, got)
		}
	}
}

func TestWaitTakesSecondsOrADuration(t *testing.T) {
	for value, want := range map[string]string{"60": "1m0s", "50": "50s", "1.5": "1.5s", "90s": "1m30s", "2m": "2m0s", "0": "0s"} {
		var s seconds
		if err := s.Set(value); err != nil || s.d.String() != want {
			t.Errorf("Set(%q) = %v, %v; want %s", value, s.d, err, want)
		}
	}
	for _, bad := range []string{"soon", "-5", "-1s", ""} {
		var s seconds
		if err := s.Set(bad); err == nil {
			t.Errorf("Set(%q) was accepted as %v", bad, s.d)
		}
	}
}

func TestTheUserIsToldHowToLinkAndTheRisk(t *testing.T) {
	t.Parallel()
	page := mcptools.Report{State: "waiting_for_scan", Page: "http://127.0.0.1:1/abc/", Detail: "for the agent"}
	raw, err := json.Marshal(linkReport{Report: page, Say: say(page)})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["state"] != page.State || out["page"] != page.Page || out["detail"] != page.Detail {
		t.Fatalf("the report lost its fields: %s", raw)
	}
	for _, want := range []string{page.Page, "Link a device", "unofficial", "keeps your messages on this computer", "model provider"} {
		if !strings.Contains(out["say"], want) {
			t.Errorf("say leaves out %q: %s", want, out["say"])
		}
	}
	code := mcptools.Report{State: "waiting_for_code", Code: "ABCD-EFGH"}
	if text := say(code); !strings.Contains(text, "ABCD-EFGH") || !strings.Contains(text, "Link with phone number instead") || !strings.Contains(text, "unofficial") {
		t.Errorf("say for a linking code: %s", text)
	}
	linked := mcptools.Report{State: "linked", Detail: "WhatsApp is linked to +40700000000."}
	if say(linked) != "" || linkText(linked) != linked.Detail {
		t.Errorf("a linked report says %q", say(linked))
	}
}

func TestSetupPrintsOneJSONObject(t *testing.T) {
	t.Parallel()
	done := setupReport{Command: "/bin/chatwire", Results: []setup.Result{{Client: "codex", Name: "Codex", Outcome: setup.Added}}}
	raw, err := json.Marshal(done)
	if err != nil {
		t.Fatal(err)
	}
	var alone map[string]any
	if err := json.Unmarshal(raw, &alone); err != nil || alone["state"] != nil || alone["results"] == nil {
		t.Fatalf("setup without linking printed %s", raw)
	}
	page := mcptools.Report{State: "waiting_for_scan", Page: "http://127.0.0.1:1/abc/"}
	done.linkReport = &linkReport{Report: page, Say: say(page)}
	raw, err = json.Marshal(done)
	if err != nil {
		t.Fatal(err)
	}
	var both map[string]any
	if err := json.Unmarshal(raw, &both); err != nil || both["state"] != page.State || both["page"] != page.Page || both["say"] == nil || both["results"] == nil {
		t.Fatalf("setup with linking printed %s", raw)
	}
}
