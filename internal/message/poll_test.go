package message_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"slices"
	"testing"
	"testing/iotest"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func lunch(selectable uint32, names ...string) *wire.Message_PollCreationMessage {
	poll := &wire.Message_PollCreationMessage{Name: new("Lunch?"), SelectableOptionsCount: new(selectable)}
	for _, name := range names {
		poll.Options = append(poll.Options, &wire.Message_PollCreationMessage_Option{OptionName: new(name)})
	}
	return poll
}

func TestPollVotesOpenOnlyForTheirBallot(t *testing.T) {
	t.Parallel()
	ballot := message.Ballot{
		Secret:  bytes.Repeat([]byte{9}, 32),
		PollID:  "3EB0POLL",
		Creator: node.JID{User: "40722222222", Server: node.ServerUser},
		Voter:   node.JID{User: "100000000000001", Server: node.ServerLID, Device: 3},
	}
	sealed, err := message.SealVote(rand.Reader, ballot, []string{"Sushi", "Pizza"})
	if err != nil {
		t.Fatal(err)
	}
	withoutDevice := ballot
	withoutDevice.Voter.Device = 0
	vote, err := message.OpenVote(withoutDevice, sealed)
	if err != nil || len(vote.GetSelectedOptions()) != 2 || !bytes.Equal(vote.GetSelectedOptions()[0], message.OptionHash("Sushi")) {
		t.Fatalf("OpenVote() = %v, %v", vote, err)
	}
	others := map[string]func(*message.Ballot){
		"another poll":   func(b *message.Ballot) { b.PollID = "3EB0OTHER" },
		"another secret": func(b *message.Ballot) { b.Secret = bytes.Repeat([]byte{8}, 32) },
		"another voter":  func(b *message.Ballot) { b.Voter = node.JID{User: "40733333333", Server: node.ServerUser} },
		"voter as phone number": func(b *message.Ballot) {
			b.Voter = node.JID{User: b.Voter.User, Server: node.ServerUser}
		},
		"another creator": func(b *message.Ballot) { b.Creator = node.JID{User: "40744444444", Server: node.ServerUser} },
	}
	for name, change := range others {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			other := ballot
			change(&other)
			if _, err := message.OpenVote(other, sealed); !errors.Is(err, message.ErrVote) {
				t.Fatalf("OpenVote() error = %v, want ErrVote", err)
			}
		})
	}
	t.Run("tampered", func(t *testing.T) {
		t.Parallel()
		payload := slices.Clone(sealed.GetEncPayload())
		payload[0] ^= 1
		if _, err := message.OpenVote(ballot, &wire.Message_PollEncValue{EncPayload: payload, EncIv: sealed.GetEncIv()}); !errors.Is(err, message.ErrVote) {
			t.Fatalf("OpenVote() error = %v, want ErrVote", err)
		}
	})
	for _, size := range []int{0, 11, 13, 16} {
		if _, err := message.OpenVote(ballot, &wire.Message_PollEncValue{EncPayload: sealed.GetEncPayload(), EncIv: make([]byte, size)}); !errors.Is(err, message.ErrVote) {
			t.Fatalf("a %d byte iv: error = %v, want ErrVote", size, err)
		}
	}
	for _, size := range []int{0, 31, 33} {
		short := ballot
		short.Secret = make([]byte, size)
		if _, err := message.OpenVote(short, sealed); !errors.Is(err, message.ErrSecret) {
			t.Fatalf("open with a %d byte secret: error = %v", size, err)
		}
		if _, err := message.SealVote(rand.Reader, short, nil); !errors.Is(err, message.ErrSecret) {
			t.Fatalf("seal with a %d byte secret: error = %v", size, err)
		}
	}
	failure := errors.New("no randomness")
	if _, err := message.SealVote(iotest.ErrReader(failure), ballot, nil); !errors.Is(err, failure) {
		t.Fatalf("SealVote() without randomness: error = %v", err)
	}
}

func TestChosenOptionsFollowThePoll(t *testing.T) {
	t.Parallel()
	hashes := func(names ...string) *wire.Message_PollVoteMessage {
		vote := &wire.Message_PollVoteMessage{}
		for _, name := range names {
			vote.SelectedOptions = append(vote.SelectedOptions, message.OptionHash(name))
		}
		return vote
	}
	tests := []struct {
		name    string
		poll    *wire.Message_PollCreationMessage
		vote    *wire.Message_PollVoteMessage
		want    []string
		wantErr error
	}{
		{name: "one of many", poll: lunch(1, "Pizza", "Sushi"), vote: hashes("Sushi"), want: []string{"Sushi"}},
		{name: "several in poll order", poll: lunch(0, "Pizza", "Sushi", "Soup"), vote: hashes("Soup", "Pizza"), want: []string{"Pizza", "Soup"}},
		{name: "every option", poll: lunch(0, "Pizza", "Sushi"), vote: hashes("Pizza", "Sushi"), want: []string{"Pizza", "Sushi"}},
		{name: "exactly the limit", poll: lunch(2, "Pizza", "Sushi", "Soup"), vote: hashes("Pizza", "Soup"), want: []string{"Pizza", "Soup"}},
		{name: "vote taken back", poll: lunch(1, "Pizza", "Sushi"), vote: hashes(), want: nil},
		{name: "the same option twice", poll: lunch(0, "Pizza", "Sushi"), vote: hashes("Pizza", "Pizza"), want: []string{"Pizza"}},
		{name: "unicode names", poll: lunch(1, "Mâncare", "寿司"), vote: hashes("寿司"), want: []string{"寿司"}},
		{name: "over the limit", poll: lunch(1, "Pizza", "Sushi"), vote: hashes("Pizza", "Sushi"), wantErr: message.ErrChoice},
		{name: "more than the options", poll: lunch(0, "Pizza"), vote: hashes("Pizza", "Pizza"), wantErr: message.ErrChoice},
		{name: "an unknown option", poll: lunch(0, "Pizza", "Sushi"), vote: hashes("Soup"), wantErr: message.ErrChoice},
		{name: "a known and an unknown option", poll: lunch(0, "Pizza", "Sushi"), vote: hashes("Pizza", "Soup"), wantErr: message.ErrChoice},
		{name: "a short hash", poll: lunch(0, "Pizza"), vote: &wire.Message_PollVoteMessage{SelectedOptions: [][]byte{message.OptionHash("Pizza")[:31]}}, wantErr: message.ErrChoice},
		{name: "a long hash", poll: lunch(0, "Pizza"), vote: &wire.Message_PollVoteMessage{SelectedOptions: [][]byte{append(message.OptionHash("Pizza"), 0)}}, wantErr: message.ErrChoice},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := message.Chosen(tt.poll, tt.vote)
			if !errors.Is(err, tt.wantErr) || !slices.Equal(got, tt.want) {
				t.Fatalf("Chosen() = %q, %v; want %q, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestPollOfEveryVersion(t *testing.T) {
	t.Parallel()
	poll := &wire.Message_PollCreationMessage{Name: new("Lunch?")}
	for _, tt := range []struct {
		name string
		m    *wire.Message
	}{
		{"v1", &wire.Message{PollCreationMessage: poll}},
		{"v2", &wire.Message{PollCreationMessageV2: poll}},
		{"v3", &wire.Message{PollCreationMessageV3: poll}},
		{"v5", &wire.Message{PollCreationMessageV5: poll}},
		{"v6", &wire.Message{PollCreationMessageV6: poll}},
	} {
		if got := message.PollOf(tt.m); got != poll {
			t.Errorf("%s: PollOf() = %v", tt.name, got)
		}
	}
	for _, m := range []*wire.Message{nil, {Conversation: new("hi")}, {PollUpdateMessage: &wire.Message_PollUpdateMessage{}}} {
		if got := message.PollOf(m); got != nil {
			t.Errorf("PollOf(%v) = %v, want nil", m, got)
		}
	}
}
