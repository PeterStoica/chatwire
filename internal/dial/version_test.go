package dial_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/signon"
)

func TestLatestVersion(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		want   uint32
		live   bool
	}{
		{name: "revision in page", body: `x"client_revision":1048570357,y`, status: http.StatusOK, want: 1048570357, live: true},
		{name: "no revision", body: `nothing`, status: http.StatusOK, want: 1047769893},
		{name: "revision too large", body: `"client_revision":4294967296,`, status: http.StatusOK, want: 1047769893},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") == "" || r.Header.Get("Sec-Fetch-Mode") != "navigate" {
					http.Error(w, "not a browser", http.StatusForbidden)
					return
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			version, source := dial.LatestVersion(t.Context(), server.Client(), server.URL)
			if version != (signon.Version{Primary: 2, Secondary: 3000, Tertiary: tt.want}) {
				t.Fatalf("version = %v", version)
			}
			if live := source == "live from "+server.URL; live != tt.live {
				t.Fatalf("source = %q", source)
			}
		})
	}
	if version, source := dial.LatestVersion(t.Context(), http.DefaultClient, "http://127.0.0.1:1"); version.Tertiary != 1047769893 || source == "" {
		t.Fatalf("unreachable page = %v, %q", version, source)
	}
}

func TestTheVersionStaysFreshAndSurvivesAnOutage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var (
			mu       sync.Mutex
			revision = "1048570357"
			fetches  int
		)
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			fetches++
			if revision == "" {
				http.Error(w, "down", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`"client_revision":` + revision + `,`))
		}))
		set := func(r string) {
			mu.Lock()
			defer mu.Unlock()
			revision = r
		}
		count := func() int {
			mu.Lock()
			defer mu.Unlock()
			return fetches
		}
		v := dial.NewVersions(server.Client(), server.URL, time.Second)
		if got := v.Current().Tertiary; got != 1048570357 {
			t.Fatalf("first version = %d", got)
		}
		time.Sleep(23 * time.Hour)
		set("1048999999")
		if got := v.Current().Tertiary; got != 1048570357 || count() != 1 {
			t.Fatalf("within a day: %d after %d fetches", got, count())
		}
		time.Sleep(2 * time.Hour)
		if got := v.Current().Tertiary; got != 1048999999 || count() != 2 {
			t.Fatalf("after a day: %d after %d fetches", got, count())
		}
		set("")
		v.Refresh()
		if got := v.Current().Tertiary; got != 1048999999 || count() != 3 {
			t.Fatalf("an outage replaced the known version: %d after %d fetches", got, count())
		}
		set("1049000001")
		time.Sleep(61 * time.Minute)
		if got := v.Current().Tertiary; got != 1049000001 || count() != 4 {
			t.Fatalf("an hour after the outage: %d after %d fetches", got, count())
		}
	})
}
