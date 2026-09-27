package dial

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/PeterStoica/chatwire/internal/signon"
)

const (
	fallbackRevision = 1047769893
	maxPage          = 4 << 20
	revisionPattern  = `"client_revision":(\d+),`
)

func LatestVersion(ctx context.Context, client *http.Client, page string) (signon.Version, string) {
	revision, err := fetchRevision(ctx, client, page)
	if err != nil {
		return signon.Version{Primary: 2, Secondary: 3000, Tertiary: fallbackRevision}, "fallback: " + err.Error()
	}
	return signon.Version{Primary: 2, Secondary: 3000, Tertiary: revision}, "live from " + page
}

func fetchRevision(ctx context.Context, client *http.Client, page string) (uint32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPage))
	if err != nil {
		return 0, err
	}
	match := regexp.MustCompile(revisionPattern).FindSubmatch(body)
	if match == nil {
		return 0, fmt.Errorf("client_revision not in page, status %d", resp.StatusCode)
	}
	revision, err := strconv.ParseUint(string(match[1]), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(revision), nil
}

type Versions struct {
	client  *http.Client
	page    string
	timeout time.Duration
	mu      sync.Mutex
	current signon.Version
	fetched bool
}

func NewVersions(client *http.Client, page string, timeout time.Duration) *Versions {
	return &Versions{client: client, page: page, timeout: timeout}
}

func (v *Versions) Current() signon.Version {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.fetched {
		v.fetchLocked()
	}
	return v.current
}

func (v *Versions) Refresh() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.fetchLocked()
}

func (v *Versions) fetchLocked() {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()
	v.current, _ = LatestVersion(ctx, v.client, v.page)
	v.fetched = true
}
