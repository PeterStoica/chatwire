package media_test

import (
	"errors"
	"maps"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/node"
)

func connReply(kind string, conn node.Node) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text(kind)}}, Children: []node.Node{conn}}
}

func mediaConn(attrs map[string]string, hosts ...node.Node) node.Node {
	n := node.Node{Tag: "media_conn", Children: hosts}
	for _, key := range []string{"auth", "ttl", "auth_ttl"} {
		if value, ok := attrs[key]; ok {
			n.Attrs = append(n.Attrs, node.Attr{Key: key, Value: node.Text(value)})
		}
	}
	return n
}

func host(name, kind string) node.Node {
	n := node.Node{Tag: "host"}
	if name != "" {
		n.Attrs = append(n.Attrs, node.Attr{Key: "hostname", Value: node.Text(name)})
	}
	if kind != "" {
		n.Attrs = append(n.Attrs, node.Attr{Key: "type", Value: node.Text(kind)})
	}
	return n
}

func TestConnRequestAsksForMediaHosts(t *testing.T) {
	t.Parallel()
	request := media.ConnRequest()
	xmlns, _ := request.Attr("xmlns").Text()
	kind, _ := request.Attr("type").Text()
	if _, ok := request.Child("media_conn"); request.Tag != "iq" || xmlns != "w:m" || kind != "set" || !ok {
		t.Fatalf("request = %s", request)
	}
}

func TestParsingMediaHosts(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_790_000_000, 0)
	valid := map[string]string{"auth": "AUTH", "ttl": "300", "auth_ttl": "21600"}
	conn, err := media.ParseConn(connReply("result", mediaConn(valid,
		host("mmg.whatsapp.net", "primary"), node.Node{Tag: "ignored"}, host("", "primary"), host("media-fallback.fna.whatsapp.net", "fallback"),
	)), now)
	if err != nil {
		t.Fatal(err)
	}
	want := []media.Host{{Hostname: "mmg.whatsapp.net"}, {Hostname: "media-fallback.fna.whatsapp.net", Fallback: true}}
	if conn.Auth != "AUTH" || conn.TTL != 300*time.Second || conn.AuthTTL != 6*time.Hour || !conn.Fetched.Equal(now) || len(conn.Hosts) != 2 || conn.Hosts[0] != want[0] || conn.Hosts[1] != want[1] {
		t.Fatalf("conn = %+v", conn)
	}
	if !conn.Fresh(now.Add(299*time.Second)) || conn.Fresh(now.Add(300*time.Second)) {
		t.Fatal("hosts must be fresh for exactly their ttl")
	}
	if (media.Conn{TTL: time.Hour, Fetched: now}).Fresh(now) {
		t.Fatal("a conn without hosts is never fresh")
	}
	zero, err := media.ParseConn(connReply("result", mediaConn(map[string]string{"auth": "A", "ttl": "0", "auth_ttl": "0"}, host("h", ""))), now)
	if err != nil || zero.Fresh(now) {
		t.Fatalf("a zero ttl is valid and never fresh: %+v, %v", zero, err)
	}
}

func TestRefusingBadMediaHostReplies(t *testing.T) {
	t.Parallel()
	valid := map[string]string{"auth": "AUTH", "ttl": "300", "auth_ttl": "21600"}
	with := func(key, value string) map[string]string {
		attrs := maps.Clone(valid)
		if value == "" {
			delete(attrs, key)
		} else {
			attrs[key] = value
		}
		return attrs
	}
	tests := []struct {
		name  string
		reply node.Node
	}{
		{name: "error reply", reply: connReply("error", mediaConn(valid, host("h", "")))},
		{name: "not an iq", reply: node.Node{Tag: "message", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}, Children: []node.Node{mediaConn(valid, host("h", ""))}}},
		{name: "no media_conn", reply: connReply("result", node.Node{Tag: "other"})},
		{name: "ttl not a number", reply: connReply("result", mediaConn(with("ttl", "soon"), host("h", "")))},
		{name: "negative ttl", reply: connReply("result", mediaConn(with("ttl", "-1"), host("h", "")))},
		{name: "negative auth ttl", reply: connReply("result", mediaConn(with("auth_ttl", "-1"), host("h", "")))},
		{name: "missing auth ttl", reply: connReply("result", mediaConn(with("auth_ttl", ""), host("h", "")))},
		{name: "no auth", reply: connReply("result", mediaConn(with("auth", ""), host("h", "")))},
		{name: "no usable host", reply: connReply("result", mediaConn(valid, host("", ""), node.Node{Tag: "other"}))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := media.ParseConn(tt.reply, time.Now()); !errors.Is(err, media.ErrConn) {
				t.Fatalf("ParseConn() error = %v", err)
			}
		})
	}
}

func TestDownloadAddresses(t *testing.T) {
	t.Parallel()
	address, err := media.DownloadURL("mmg.whatsapp.net", "/v/t62.7118-24/x.enc?ccb=11-4&oh=01AB", []byte{0xfb, 0xff}, media.Image)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "mmg.whatsapp.net" || parsed.Path != "/v/t62.7118-24/x.enc" ||
		query.Get("hash") != "-_8=" || query.Get("mms-type") != "image" || !query.Has("__wa-mms") || query.Get("ccb") != "11-4" || query.Get("oh") != "01AB" {
		t.Fatalf("address = %s", address)
	}
	unhashed, err := media.DownloadURL("mmg.whatsapp.net", "/v/x.enc", nil, media.Voice)
	if err != nil {
		t.Fatal(err)
	}
	if parsed, _ := url.Parse(unhashed); parsed.Query().Has("hash") || parsed.Query().Get("mms-type") != "ptt" {
		t.Fatalf("address without a hash = %s", unhashed)
	}
	for _, path := range []string{"https://attacker.example/x", "//attacker.example/x", "http://mmg.whatsapp.net/x", "/v/%zz"} {
		if _, err := media.DownloadURL("mmg.whatsapp.net", path, nil, media.Image); !errors.Is(err, media.ErrPath) {
			t.Errorf("DownloadURL(%q) error = %v", path, err)
		}
	}
}

func TestUploadAddresses(t *testing.T) {
	t.Parallel()
	address := media.UploadURL("mmg.whatsapp.net", media.Image, []byte{0xfb, 0xff, 0x01}, "AU/TH+=", 42)
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "mmg.whatsapp.net" || parsed.Path != "/mms/image/-_8B" || query.Get("auth") != "AU/TH+=" ||
		query.Get("token") != "-_8B" || query.Get("media_id") != "42" || !strings.HasPrefix(parsed.RawQuery, "auth=") {
		t.Fatalf("address = %s", address)
	}
	if padded := media.UploadURL("h", media.Document, []byte{1}, "a", 1); !strings.Contains(padded, "/mms/document/AQ==?") || !strings.Contains(padded, "token=AQ%3D%3D") {
		t.Fatalf("padding must survive: %s", padded)
	}
	if browser := media.UploadURL("h", media.Audio, []byte{1}, "az AZ09*-._~é/", 1); !strings.Contains(browser, "auth=az+AZ09*-._%7E%C3%A9%2F&") {
		t.Fatalf("auth must be form-encoded like a browser: %s", browser)
	}
}
