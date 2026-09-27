package message

import (
	"cmp"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	pollVoteUseCase = "Poll Vote"
	SecretSize      = 32
	voteIVSize      = 12
)

var (
	ErrSecret = errors.New("message: the poll's message secret must be 32 bytes")
	ErrVote   = errors.New("message: poll vote does not decrypt")
	ErrChoice = errors.New("message: poll vote picks options the poll does not have")
)

type Ballot struct {
	Secret  []byte
	PollID  string
	Creator node.JID
	Voter   node.JID
}

func PollOf(m *wire.Message) *wire.Message_PollCreationMessage {
	if wrapped := m.GetPollCreationMessageV4().GetMessage(); wrapped != nil {
		return PollOf(wrapped)
	}
	return cmp.Or(m.GetPollCreationMessage(), m.GetPollCreationMessageV2(), m.GetPollCreationMessageV3(), m.GetPollCreationMessageV5(), m.GetPollCreationMessageV6())
}

func OptionHash(name string) []byte {
	sum := sha256.Sum256([]byte(name))
	return sum[:]
}

func (b Ballot) sealer() (cipher.AEAD, error) {
	return addonCipher(b.Secret, b.PollID, b.Creator, b.Voter, pollVoteUseCase)
}

func (b Ballot) additionalData() []byte {
	return []byte(b.PollID + "\x00" + b.Voter.WithoutDevice().String())
}

func SealVote(random io.Reader, b Ballot, options []string) (*wire.Message_PollEncValue, error) {
	aead, err := b.sealer()
	if err != nil {
		return nil, err
	}
	vote := &wire.Message_PollVoteMessage{}
	for _, option := range options {
		vote.SelectedOptions = append(vote.SelectedOptions, OptionHash(option))
	}
	plain, err := proto.Marshal(vote)
	if err != nil {
		return nil, fmt.Errorf("message: encode vote: %w", err)
	}
	iv := make([]byte, voteIVSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return nil, fmt.Errorf("message: vote iv: %w", err)
	}
	return &wire.Message_PollEncValue{EncPayload: aead.Seal(nil, iv, plain, b.additionalData()), EncIv: iv}, nil
}

func OpenVote(b Ballot, enc *wire.Message_PollEncValue) (*wire.Message_PollVoteMessage, error) {
	aead, err := b.sealer()
	if err != nil {
		return nil, err
	}
	if len(enc.GetEncIv()) != aead.NonceSize() {
		return nil, fmt.Errorf("%w: iv of %d bytes", ErrVote, len(enc.GetEncIv()))
	}
	plain, err := aead.Open(nil, enc.GetEncIv(), enc.GetEncPayload(), b.additionalData())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVote, err)
	}
	vote := &wire.Message_PollVoteMessage{}
	if err := proto.Unmarshal(plain, vote); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVote, err)
	}
	return vote, nil
}

func Chosen(poll *wire.Message_PollCreationMessage, vote *wire.Message_PollVoteMessage) ([]string, error) {
	selected, options := vote.GetSelectedOptions(), poll.GetOptions()
	if limit := int(poll.GetSelectableOptionsCount()); len(selected) > len(options) || limit != 0 && len(selected) > limit {
		return nil, fmt.Errorf("%w: %d picked of %d", ErrChoice, len(selected), len(options))
	}
	picked := make(map[[sha256.Size]byte]bool, len(selected))
	for _, hash := range selected {
		if len(hash) != sha256.Size {
			return nil, fmt.Errorf("%w: a %d byte option hash", ErrChoice, len(hash))
		}
		picked[[sha256.Size]byte(hash)] = true
	}
	var chosen []string
	for _, option := range options {
		if hash := [sha256.Size]byte(OptionHash(option.GetOptionName())); picked[hash] {
			chosen = append(chosen, option.GetOptionName())
			delete(picked, hash)
		}
	}
	if len(picked) > 0 {
		return nil, fmt.Errorf("%w: %d unknown options", ErrChoice, len(picked))
	}
	return chosen, nil
}

const (
	MaxPollQuestion = 255
	MaxPollOptions  = 12
	MinPollOptions  = 2
	MaxPollOption   = 100
)

func NewPoll(question string, options []string, multiple bool) *wire.Message {
	selectable := uint32(1)
	if multiple {
		selectable = 0
	}
	poll := &wire.Message_PollCreationMessage{
		Name: new(question), SelectableOptionsCount: new(selectable),
		PollContentType: wire.Message_TEXT.Enum(), PollType: wire.Message_POLL.Enum(),
	}
	for _, o := range options {
		poll.Options = append(poll.Options, &wire.Message_PollCreationMessage_Option{OptionName: new(o)})
	}
	if multiple {
		return &wire.Message{PollCreationMessage: poll}
	}
	return &wire.Message{PollCreationMessageV3: poll}
}

var ErrPoll = errors.New("message: not a valid poll")

type PollProblem struct {
	Reason string
}

func (p PollProblem) Error() string {
	return fmt.Sprintf("%v: %s", ErrPoll, p.Reason)
}

func (p PollProblem) Is(target error) bool {
	return target == ErrPoll
}

func CheckPoll(question string, options []string) (string, []string, error) {
	question = strings.TrimSpace(question)
	if question == "" || jsLength(question) > MaxPollQuestion {
		return "", nil, PollProblem{Reason: fmt.Sprintf("the question must be 1 to %d characters", MaxPollQuestion)}
	}
	if len(options) < MinPollOptions || len(options) > MaxPollOptions {
		return "", nil, PollProblem{Reason: fmt.Sprintf("give %d to %d options, not %d", MinPollOptions, MaxPollOptions, len(options))}
	}
	seen := make(map[string]bool, len(options))
	out := make([]string, 0, len(options))
	for _, o := range options {
		o = strings.TrimSpace(o)
		key := strings.ToLower(o)
		switch {
		case o == "" || jsLength(o) > MaxPollOption:
			return "", nil, PollProblem{Reason: fmt.Sprintf("each option must be 1 to %d characters (%q)", MaxPollOption, o)}
		case seen[key]:
			return "", nil, PollProblem{Reason: fmt.Sprintf("%q is given twice", o)}
		}
		seen[key] = true
		out = append(out, o)
	}
	return question, out, nil
}

func jsLength(s string) int {
	return len(utf16.Encode([]rune(s)))
}
