package linkpage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"rsc.io/qr"

	"github.com/PeterStoica/chatwire/internal/linker"
	"github.com/PeterStoica/chatwire/internal/linkpage"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

type fakeLinker struct {
	mu      sync.Mutex
	status  linker.Status
	started []string
}

func (f *fakeLinker) Status() linker.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeLinker) Start(_ context.Context, phone string) (linker.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, phone)
	return f.status, nil
}

func (f *fakeLinker) set(st linker.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = st
}

func (f *fakeLinker) starts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.started...)
}

func open(t *testing.T) (*fakeLinker, *linkpage.Page) {
	t.Helper()
	l := &fakeLinker{}
	p, err := linkpage.Start(t.Context(), l, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return l, p
}

type reply struct {
	code   int
	header http.Header
	body   []byte
}

func get(t *testing.T, url string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return do(t, req)
}

func do(t *testing.T, req *http.Request) reply {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{code: resp.StatusCode, header: resp.Header, body: body}
}

func state(t *testing.T, p *linkpage.Page) linkpage.State {
	t.Helper()
	r := get(t, p.URL+"state")
	var st linkpage.State
	if r.code != http.StatusOK || json.Unmarshal(r.body, &st) != nil {
		t.Fatalf("state: %d %s", r.code, r.body)
	}
	return st
}

func TestThePageIsLocalAndPrivate(t *testing.T) {
	t.Parallel()
	_, p := open(t)
	if !strings.HasPrefix(p.URL, "http://127.0.0.1:") || len(strings.Split(p.URL, "/")[3]) != 32 {
		t.Fatalf("URL = %s", p.URL)
	}
	page := get(t, p.URL)
	if page.code != http.StatusOK || !bytes.Contains(page.body, []byte("Link your WhatsApp")) || !bytes.Contains(page.body, []byte("Linked Devices")) ||
		page.header.Get("Cache-Control") != "no-store" || !strings.Contains(page.header.Get("Content-Security-Policy"), "default-src 'none'") ||
		page.header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("page: %d %v", page.code, page.header)
	}
	root := strings.TrimSuffix(p.URL, strings.Split(p.URL, "/")[3]+"/")
	if r := get(t, root); r.code != http.StatusNotFound {
		t.Fatalf("the page answered without its token: %d", r.code)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, p.URL+"state", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	if r := do(t, req); r.code != http.StatusMisdirectedRequest {
		t.Fatalf("a request for another host got %d", r.code)
	}
	failure := errors.New("no randomness")
	if _, err := linkpage.Start(t.Context(), &fakeLinker{}, iotest.ErrReader(failure)); !errors.Is(err, linkpage.ErrToken) || !errors.Is(err, failure) {
		t.Fatalf("Start without randomness: %v", err)
	}
}

func TestThePageFollowsTheLinker(t *testing.T) {
	t.Parallel()
	l, p := open(t)
	if st := state(t, p); st.Phase != "idle" {
		t.Fatalf("before linking: %+v", st)
	}
	if r := get(t, p.URL+"qr.png"); r.code != http.StatusNotFound {
		t.Fatalf("a QR image before there is one: %d", r.code)
	}
	l.set(linker.Status{Phase: linker.ShowingQR, QR: "https://wa.me/settings/linked_devices#2@ref-1,abc"})
	first := state(t, p)
	if first.Phase != "qr" || len(first.QR) != 12 || state(t, p).QR != first.QR {
		t.Fatalf("showing a QR: %+v", first)
	}
	image := get(t, p.URL+"qr.png")
	want, err := qr.Encode("https://wa.me/settings/linked_devices#2@ref-1,abc", qr.M)
	if err != nil {
		t.Fatal(err)
	}
	want.Scale = 8
	if image.code != http.StatusOK || image.header.Get("Content-Type") != "image/png" || !bytes.Equal(image.body, want.PNG()) {
		t.Fatalf("qr.png: %d %s, %d bytes", image.code, image.header.Get("Content-Type"), len(image.body))
	}
	l.set(linker.Status{Phase: linker.ShowingQR, QR: "https://wa.me/settings/linked_devices#2@ref-2,abc"})
	if next := state(t, p); next.QR == first.QR {
		t.Fatalf("the QR rotated but its id did not: %+v", next)
	}
	if again := get(t, p.URL+"qr.png"); bytes.Equal(again.body, image.body) {
		t.Fatal("the image did not follow the new QR")
	}
	for _, tt := range []struct {
		status linker.Status
		want   linkpage.State
	}{
		{linker.Status{Phase: linker.Starting}, linkpage.State{Phase: "starting"}},
		{linker.Status{Phase: linker.ShowingCode, Code: "ABCD-EFGH"}, linkpage.State{Phase: "code", Code: "ABCD-EFGH"}},
		{linker.Status{Phase: linker.Linked, Account: pairing.Account{JID: node.JID{User: "40700000000", Server: node.ServerUser}}}, linkpage.State{Phase: "linked", Account: "+40700000000"}},
		{linker.Status{Phase: linker.Expired}, linkpage.State{Phase: "expired"}},
		{linker.Status{Phase: linker.Failed, Err: errors.New("the phone said no")}, linkpage.State{Phase: "failed", Problem: "the phone said no"}},
		{linker.Status{Phase: linker.Failed}, linkpage.State{Phase: "failed"}},
		{linker.Status{Phase: linker.LoggedOut, Err: errors.New("removed")}, linkpage.State{Phase: "logged_out", Problem: "removed"}},
		{linker.Status{Phase: linker.Unlinked}, linkpage.State{Phase: "idle"}},
	} {
		l.set(tt.status)
		if got := state(t, p); got != tt.want {
			t.Errorf("%v: %+v, want %+v", tt.status.Phase, got, tt.want)
		}
	}
}

func TestANewCodeOnlyWhenLinkingStopped(t *testing.T) {
	t.Parallel()
	l, p := open(t)
	post := func() int {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, p.URL+"again", nil)
		if err != nil {
			t.Fatal(err)
		}
		return do(t, req).code
	}
	for _, busy := range []linker.Phase{linker.Linked, linker.Starting, linker.ShowingQR, linker.ShowingCode} {
		l.set(linker.Status{Phase: busy})
		if code := post(); code != http.StatusConflict {
			t.Fatalf("again while %v = %d", busy, code)
		}
	}
	l.set(linker.Status{Phase: linker.Expired})
	if code := post(); code != http.StatusAccepted {
		t.Fatalf("again after expiry = %d", code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(l.starts()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := l.starts(); len(got) != 1 || got[0] != "" {
		t.Fatalf("started %q, want one QR link", got)
	}
	if r := get(t, p.URL+"again"); r.code != http.StatusMethodNotAllowed {
		t.Fatalf("GET again = %d", r.code)
	}
}
