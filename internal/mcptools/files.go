package mcptools

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/messenger"
)

const FilesVariable = "CHATWIRE_FILES"

func DefaultFolders() []string {
	var out []string
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{"Desktop", "Documents", "Downloads", "Pictures", "Movies", "Videos", "Music"} {
			out = append(out, filepath.Join(home, name))
		}
	}
	out = append(out, os.TempDir())
	if runtime.GOOS != "windows" {
		out = append(out, "/tmp")
	}
	return out
}

type gate struct {
	allowed []string
	private []string
	shown   []string
}

func newGate(opts Options) gate {
	folders := opts.Folders
	if len(folders) == 0 {
		folders = DefaultFolders()
	}
	if opts.MediaDir != "" {
		folders = append(folders, opts.MediaDir)
	}
	var g gate
	for _, f := range folders {
		if real, err := filepath.EvalSymlinks(f); err == nil {
			g.allowed = append(g.allowed, real)
			g.shown = append(g.shown, f)
		}
	}
	for _, f := range opts.Private {
		if real, err := filepath.EvalSymlinks(f); err == nil {
			g.private = append(g.private, real)
		}
	}
	return g
}

func within(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func deepest(roots []string, path string) (string, int) {
	best, depth := "", -1
	for _, root := range roots {
		if rel, ok := within(root, path); ok && len(root) > depth {
			best, depth = rel, len(root)
		}
	}
	return best, depth
}

func (g gate) open(path string) (messenger.File, *refusal) {
	path = strings.TrimSpace(path)
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}
	if path == "" {
		return messenger.File{}, &refusal{state: "file_not_found", detail: "Say which file to send, as a path on this computer."}
	}
	if !filepath.IsAbs(path) {
		return messenger.File{}, &refusal{state: "not_a_full_path", detail: fmt.Sprintf("Give the full path of the file, like ~/Downloads/%s.", filepath.Base(path))}
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return messenger.File{}, &refusal{state: "file_not_found", detail: fmt.Sprintf("There is no file at %s.", path)}
	}
	rel, allowedAt := deepest(g.allowed, real)
	if _, privateAt := deepest(g.private, real); allowedAt < 0 || privateAt > allowedAt {
		return messenger.File{}, &refusal{state: "folder_not_allowed", detail: fmt.Sprintf(
			"For safety, files are only sent from these folders: %s. Ask the user to move or copy it into one of them first, or to allow its folder with the %s setting.",
			strings.Join(g.shown, ", "), FilesVariable)}
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") && part != "." {
			return messenger.File{}, &refusal{state: "hidden_file", detail: "Hidden files and folders (names starting with a dot) are never sent; they often hold passwords and keys."}
		}
	}
	before, err := os.Stat(real)
	switch {
	case err != nil:
		return messenger.File{}, &refusal{state: "file_not_found", detail: fmt.Sprintf("There is no file at %s.", path)}
	case before.IsDir():
		return messenger.File{}, &refusal{state: "not_a_file", detail: fmt.Sprintf("%s is a folder; send the files inside it one by one, or zip it first.", path)}
	case !before.Mode().IsRegular():
		return messenger.File{}, &refusal{state: "not_a_file", detail: fmt.Sprintf("%s is not a regular file.", path)}
	case before.Size() > client.MaxUpload:
		return messenger.File{}, &refusal{state: "too_large", detail: fmt.Sprintf("%s is %d MB; at most %d MB can be sent.", path, before.Size()>>20, client.MaxUpload>>20)}
	}
	f, err := os.Open(real)
	if err != nil {
		return messenger.File{}, &refusal{state: stateFailed, detail: fmt.Sprintf("Could not read %s: %v", path, err)}
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return messenger.File{}, &refusal{state: stateFailed, detail: fmt.Sprintf("%s changed while it was being read; try again.", path)}
	}
	data, err := io.ReadAll(io.LimitReader(f, client.MaxUpload+1))
	switch {
	case err != nil:
		return messenger.File{}, &refusal{state: stateFailed, detail: fmt.Sprintf("Could not read %s: %v", path, err)}
	case len(data) > client.MaxUpload:
		return messenger.File{}, &refusal{state: "too_large", detail: fmt.Sprintf("%s grew past %d MB while it was being read.", path, client.MaxUpload>>20)}
	}
	return messenger.File{Name: filepath.Base(path), Data: data}, nil
}
