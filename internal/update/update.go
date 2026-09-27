package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	Home       = "https://github.com/PeterStoica/chatwire"
	maxBinary  = 200 << 20
	maxSums    = 64 << 10
	checksums  = "checksums.txt"
	tagMarker  = "/releases/tag/"
	executable = 0o755
)

var (
	ErrNoRelease = errors.New("update: no published release")
	ErrChecksum  = errors.New("update: the download does not match the release checksums")
)

func Latest(ctx context.Context, client *http.Client, home string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(home, "/")+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noFollow.Do(req)
	if err != nil {
		return "", fmt.Errorf("update: latest release: %w", err)
	}
	_ = resp.Body.Close()
	location := resp.Header.Get("Location")
	_, tag, found := strings.Cut(location, tagMarker)
	if resp.StatusCode/100 != 3 || !found || tag == "" {
		return "", fmt.Errorf("%w: %s answered %d", ErrNoRelease, home, resp.StatusCode)
	}
	return path.Base(tag), nil
}

func Newer(current, latest string) bool {
	have, ok := release(current)
	want, ok2 := release(latest)
	if !ok || !ok2 {
		return false
	}
	for i := range have {
		if want[i] != have[i] {
			return want[i] > have[i]
		}
	}
	return false
}

func IsRelease(v string) bool {
	_, ok := release(v)
	return ok
}

func release(v string) ([3]int, bool) {
	var out [3]int
	core, ok := strings.CutPrefix(v, "v")
	if !ok || strings.ContainsAny(core, "-+") {
		return out, false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func Asset(goos, goarch string) string {
	name := "chatwire_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func Download(ctx context.Context, client *http.Client, home, version string) ([]byte, error) {
	base := strings.TrimRight(home, "/") + "/releases/download/" + version + "/"
	name := Asset(runtime.GOOS, runtime.GOARCH)
	sums, err := fetch(ctx, client, base+checksums, maxSums)
	if err != nil {
		return nil, err
	}
	want, err := sumOf(sums, name)
	if err != nil {
		return nil, err
	}
	binary, err := fetch(ctx, client, base+name, maxBinary)
	if err != nil {
		return nil, err
	}
	if got := sha256.Sum256(binary); !bytes.Equal(got[:], want) {
		return nil, fmt.Errorf("%w: %s", ErrChecksum, name)
	}
	return binary, nil
}

func sumOf(sums []byte, name string) ([]byte, error) {
	lines := bufio.NewScanner(bytes.NewReader(sums))
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return hex.DecodeString(fields[0])
		}
	}
	return nil, fmt.Errorf("%w: %s is not listed", ErrChecksum, name)
}

func fetch(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: download %s: %w", path.Base(url), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: download %s: %s", path.Base(url), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("update: download %s: %w", path.Base(url), err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("update: %s is larger than %d bytes", path.Base(url), limit)
	}
	return body, nil
}

func Install(target string, binary []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".chatwire-update-*")
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: write: %w", err)
	}
	if err := tmp.Chmod(executable); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("update: write: %w", err)
	}
	if runtime.GOOS == "windows" {
		aside := target + ".old"
		_ = os.Remove(aside)
		if err := os.Rename(target, aside); err != nil {
			return fmt.Errorf("update: move the running copy aside: %w", err)
		}
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("update: replace %s: %w", target, err)
	}
	return nil
}
