package message

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	idPrefix     = "3EB0"
	idHashBytes  = 9
	idRandomSize = 16
)

func NewID(now time.Time, self node.JID, random io.Reader) (string, error) {
	var noise [idRandomSize]byte
	if _, err := io.ReadFull(random, noise[:]); err != nil {
		return "", fmt.Errorf("message: id randomness: %w", err)
	}
	material, err := binary.Append(nil, binary.BigEndian, now.Unix())
	if err != nil {
		return "", fmt.Errorf("message: id time: %w", err)
	}
	material = append(material, self.String()...)
	sum := sha256.Sum256(append(material, noise[:]...))
	return idPrefix + strings.ToUpper(hex.EncodeToString(sum[:idHashBytes])), nil
}
