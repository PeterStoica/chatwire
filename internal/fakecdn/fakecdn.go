package fakecdn

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const Auth = "fake-auth"

type CDN struct {
	mu      sync.Mutex
	files   map[string][]byte
	down    map[string]bool
	auth    string
	Uploads []Upload
}

type Upload struct {
	Type    string
	Path    string
	Query   string
	Size    int
	Refused string
}

func New() *CDN {
	return &CDN{files: map[string][]byte{}, down: map[string]bool{}, auth: Auth}
}

func (c *CDN) SetDown(host string, down bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.down[host] = down
}

func (c *CDN) ExpectAuth(auth string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = auth
}

func (c *CDN) Put(path string, file []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = file
}

func (c *CDN) File(path string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	file, ok := c.files[path]
	return file, ok
}

func (c *CDN) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Origin") != "https://web.whatsapp.com" {
		http.Error(w, "no origin", http.StatusForbidden)
		return
	}
	c.mu.Lock()
	down := c.down[req.Host]
	c.mu.Unlock()
	if down {
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	switch req.Method {
	case http.MethodGet:
		if file, ok := c.File(req.URL.Path); ok {
			_, _ = w.Write(file)
			return
		}
		http.NotFound(w, req)
	case http.MethodPost:
		c.upload(w, req)
	default:
		http.Error(w, "method", http.StatusMethodNotAllowed)
	}
}

func (c *CDN) upload(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
	record := Upload{Path: req.URL.Path, Query: req.URL.RawQuery, Size: len(body)}
	refuse := func(status int, why string) {
		record.Refused = why
		c.mu.Lock()
		c.Uploads = append(c.Uploads, record)
		c.mu.Unlock()
		http.Error(w, why, status)
	}
	if len(parts) != 3 || parts[0] != "mms" {
		refuse(http.StatusNotFound, "no such upload path")
		return
	}
	record.Type = parts[1]
	sum := sha256.Sum256(body)
	hash := strings.NewReplacer("+", "-", "/", "_").Replace(base64.StdEncoding.EncodeToString(sum[:]))
	query := req.URL.Query()
	c.mu.Lock()
	auth := c.auth
	c.mu.Unlock()
	switch {
	case query.Get("auth") != auth:
		refuse(http.StatusUnauthorized, "bad auth")
		return
	case parts[2] != hash || query.Get("token") != hash:
		refuse(http.StatusUnsupportedMediaType, "hash mismatch")
		return
	case query.Get("media_id") == "":
		refuse(http.StatusBadRequest, "no media id")
		return
	}
	direct := fmt.Sprintf("/v/t62.%s/%s.enc?ccb=11-4&oh=01AB", record.Type, hash[:12])
	c.mu.Lock()
	c.files[strings.SplitN(direct, "?", 2)[0]] = body
	c.Uploads = append(c.Uploads, record)
	c.mu.Unlock()
	reply, err := json.Marshal(map[string]string{"url": "https://" + req.Host + direct, "direct_path": direct})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(reply)
}
