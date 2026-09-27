package mcpapp

import (
	"strings"
	"testing"
)

func TestReportsReadAsCommands(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ detail, want string }{
		{"The linking code or QR code expired before it was used. Call link_whatsapp again for a fresh one.", "Run chatwire link again for a fresh one."},
		{"Linking failed: the phone said no. Call link_whatsapp to try again.", "Linking failed: the phone said no. Run chatwire link to try again."},
		{"WhatsApp is not linked yet. Ask the user for their WhatsApp mobile number with country code and call link_whatsapp with it.", "WhatsApp is not linked yet. Run chatwire link to link it."},
		{"WhatsApp unlinked this computer. Ask the user for their WhatsApp mobile number with country code and call link_whatsapp to link again.", "WhatsApp unlinked this computer. Run chatwire link to link it again."},
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
