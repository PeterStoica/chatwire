package message

import (
	"bytes"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestAVoteThatIsNotAVoteIsRefused(t *testing.T) {
	t.Parallel()
	ballot := Ballot{Secret: bytes.Repeat([]byte{1}, 32), PollID: "3EB0", Creator: node.JID{User: "1", Server: node.ServerUser}, Voter: node.JID{User: "2", Server: node.ServerUser}}
	aead, err := ballot.sealer()
	if err != nil {
		t.Fatal(err)
	}
	iv := make([]byte, voteIVSize)
	enc := &wire.Message_PollEncValue{EncPayload: aead.Seal(nil, iv, []byte{0xff}, ballot.additionalData()), EncIv: iv}
	if _, err := OpenVote(ballot, enc); !errors.Is(err, ErrVote) {
		t.Fatalf("OpenVote() error = %v, want ErrVote", err)
	}
}
