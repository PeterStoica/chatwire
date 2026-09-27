package notices_test

import (
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/notices"
)

func TestTheProgramCarriesEveryLicence(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"Copyright 2026 QA DNA", "Apache License", "== modernc.org/libc/LICENSE-3RD-PARTY.md", "== golang.org/x/text/PATENTS", "== github.com/segmentio/asm/LICENSE", "== github.com/modelcontextprotocol/go-sdk/LICENSE"} {
		if !strings.Contains(notices.Text, want) {
			t.Errorf("the licences leave out %q", want)
		}
	}
}
