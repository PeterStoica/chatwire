package dial_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
