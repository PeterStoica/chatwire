package mcptools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnlyFilesFromAllowedFoldersAreSent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	write := func(rel, content string) string {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	report := write("Documents/report.pdf", "report")
	write("Documents/.env", "TOKEN=1")
	write("Documents/.keys/id", "key")
	write("state/linked.json", "keys")
	photo := write("state/media/photo.jpg", "photo")
	secret := write("elsewhere/id_rsa", "private key")
	if err := os.Symlink(secret, filepath.Join(root, "Documents", "innocent.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(report, filepath.Join(root, "Desktop-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Documents", "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	g := newGate(Options{Folders: []string{filepath.Join(root, "Documents")}, MediaDir: filepath.Join(root, "state", "media"), Private: []string{filepath.Join(root, "state")}})
	for _, tt := range []struct {
		name      string
		path      string
		wantState string
		wantData  string
	}{
		{name: "a document", path: report, wantData: "report"},
		{name: "a document by ~/", path: "~/Documents/report.pdf", wantData: "report"},
		{name: "a link from elsewhere to an allowed file", path: filepath.Join(root, "Desktop-link"), wantData: "report"},
		{name: "downloaded media", path: photo, wantData: "photo"},
		{name: "a file outside the allowed folders", path: secret, wantState: "folder_not_allowed"},
		{name: "a link inside that points outside", path: filepath.Join(root, "Documents", "innocent.txt"), wantState: "folder_not_allowed"},
		{name: "a way out with ..", path: filepath.Join(root, "Documents", "..", "elsewhere", "id_rsa"), wantState: "folder_not_allowed"},
		{name: "chatwire's own keys", path: filepath.Join(root, "state", "linked.json"), wantState: "folder_not_allowed"},
		{name: "a hidden file", path: filepath.Join(root, "Documents", ".env"), wantState: "hidden_file"},
		{name: "a file in a hidden folder", path: filepath.Join(root, "Documents", ".keys", "id"), wantState: "hidden_file"},
		{name: "a folder", path: filepath.Join(root, "Documents", "folder"), wantState: "not_a_file"},
		{name: "a relative path", path: "Documents/report.pdf", wantState: "not_a_full_path"},
		{name: "nothing there", path: filepath.Join(root, "Documents", "missing.pdf"), wantState: "file_not_found"},
		{name: "no path", path: " ", wantState: "file_not_found"},
	} {
		f, refused := g.open(tt.path)
		switch {
		case tt.wantState == "" && refused != nil:
			t.Errorf("%s: refused %s: %s", tt.name, refused.state, refused.detail)
		case tt.wantState == "" && string(f.Data) != tt.wantData:
			t.Errorf("%s: read %q", tt.name, f.Data)
		case tt.wantState != "" && (refused == nil || refused.state != tt.wantState):
			t.Errorf("%s: got %+v, %+v; want %s", tt.name, f, refused, tt.wantState)
		}
	}
	_, refused := g.open(secret)
	if refused == nil || !strings.Contains(refused.detail, filepath.Join(root, "Documents")) || !strings.Contains(refused.detail, FilesVariable) {
		t.Fatalf("the refusal does not say where files can come from: %+v", refused)
	}
}
