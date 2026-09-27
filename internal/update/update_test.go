package update_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PeterStoica/chatwire/internal/update"
)

func fakeGitHub(t *testing.T, binary []byte, sums string) *httptest.Server {
	t.Helper()
	name := update.Asset(runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	mux.HandleFunc("/owner/chatwire/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/owner/chatwire/releases/tag/v1.4.0", http.StatusFound)
	})
	mux.HandleFunc("/owner/chatwire/releases/download/v1.4.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	mux.HandleFunc("/owner/chatwire/releases/download/v1.4.0/"+name, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binary)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestFindingAndFetchingANewRelease(t *testing.T) {
	binary := []byte("the new chatwire")
	sum := sha256.Sum256(binary)
	name := update.Asset(runtime.GOOS, runtime.GOARCH)
	server := fakeGitHub(t, binary, hex.EncodeToString(sum[:])+"  "+name+"\n0000  chatwire_other_os\n")
	home := server.URL + "/owner/chatwire"
	latest, err := update.Latest(t.Context(), server.Client(), home)
	if err != nil || latest != "v1.4.0" {
		t.Fatalf("Latest = %q, %v", latest, err)
	}
	got, err := update.Download(t.Context(), server.Client(), home, latest)
	if err != nil || string(got) != string(binary) {
		t.Fatalf("Download = %q, %v", got, err)
	}
	tampered := fakeGitHub(t, []byte("something else"), hex.EncodeToString(sum[:])+"  "+name+"\n")
	if _, err := update.Download(t.Context(), tampered.Client(), tampered.URL+"/owner/chatwire", "v1.4.0"); !errors.Is(err, update.ErrChecksum) {
		t.Fatalf("a download that does not match its checksum: %v", err)
	}
	unlisted := fakeGitHub(t, binary, "0000  chatwire_other_os\n")
	if _, err := update.Download(t.Context(), unlisted.Client(), unlisted.URL+"/owner/chatwire", "v1.4.0"); !errors.Is(err, update.ErrChecksum) {
		t.Fatalf("a build missing from the checksums: %v", err)
	}
	if _, err := update.Latest(t.Context(), server.Client(), server.URL+"/nobody/private"); !errors.Is(err, update.ErrNoRelease) {
		t.Fatalf("no releases: %v", err)
	}
}

func TestWhichVersionsAreNewer(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"v1.2.3", "v1.2.4", true},
		{"v1.2.3", "v1.10.0", true},
		{"v1.2.3", "v2.0.0", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.3.0", "v1.2.9", false},
		{"dev", "v1.0.0", false},
		{"v0.0.0-20260927130517-b468c5e358a3", "v1.0.0", false},
		{"v1.2.3", "v1.3.0-rc1", false},
		{"v1.2.3+dirty", "v1.3.0", false},
	} {
		if got := update.Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%s, %s) = %v", tt.current, tt.latest, got)
		}
	}
}

func TestInstallingReplacesTheBinaryInPlace(t *testing.T) {
	target := filepath.Join(t.TempDir(), "chatwire")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := update.Install(target, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	info, statErr := os.Stat(target)
	if err != nil || statErr != nil || string(got) != "new" || runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("after installing: %q, %v, %v", got, err, statErr)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".chatwire-update-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}
