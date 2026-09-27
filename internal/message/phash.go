package message

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"strconv"
	"strings"

	"github.com/PeterStoica/chatwire/internal/node"
)

func Phash(devices []node.JID) string {
	full := make([]string, 0, len(devices))
	for _, d := range devices {
		full = append(full, d.User+".0:"+strconv.Itoa(int(d.Device))+"@"+string(d.Server))
	}
	slices.Sort(full)
	sum := sha256.Sum256([]byte(strings.Join(full, "")))
	return "2:" + base64.StdEncoding.EncodeToString(sum[:6])
}
