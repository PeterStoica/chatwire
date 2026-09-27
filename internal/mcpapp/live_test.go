//go:build live

package mcpapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLiveQRLinkingThroughTheBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mcp")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../../cmd/chatwire")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	transport := &mcp.CommandTransport{Command: exec.CommandContext(ctx, binary, "-state", filepath.Join(t.TempDir(), "linked.json"))}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "live-test", Version: "0"}, nil).Connect(ctx, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	state := func(result *mcp.CallToolResult) string {
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			State string `json:"state"`
		}
		_ = json.Unmarshal(raw, &report)
		return report.State
	}
	status, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "whatsapp_status"})
	if err != nil || state(status) != "not_linked" {
		t.Fatalf("whatsapp_status = %v, %v", status, err)
	}
	start := time.Now()
	link, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "link_whatsapp", Arguments: map[string]any{}})
	if err != nil || state(link) != "waiting_for_scan" {
		t.Fatalf("link_whatsapp = %+v, %v", link, err)
	}
	for _, content := range link.Content {
		if image, ok := content.(*mcp.ImageContent); ok {
			decoded, err := png.Decode(bytes.NewReader(image.Data))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("real WhatsApp QR returned in %s: %dx%d PNG, %d bytes", time.Since(start).Round(time.Millisecond), decoded.Bounds().Dx(), decoded.Bounds().Dy(), len(image.Data))
			return
		}
	}
	t.Fatal("no QR image in the result")
}
