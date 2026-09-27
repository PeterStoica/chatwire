package limits

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	opTimelock      = "NotificationUserReachoutTimelockUpdate"
	opCap           = "MessageCappingInfoNotification"
	defaultTimelock = time.Minute
)

var ErrMalformed = errors.New("limits: malformed account limit notification")

type Timelock struct {
	Active bool
	Ends   time.Time
	Kind   string
}

func (t Timelock) On(now time.Time) bool {
	return t.Active && now.Before(t.Ends)
}

type CapStatus string

const (
	NoCap         CapStatus = "NONE"
	FirstWarning  CapStatus = "FIRST_WARNING"
	SecondWarning CapStatus = "SECOND_WARNING"
	Capped        CapStatus = "CAPPED"
)

type Cap struct {
	Status CapStatus
	Used   int
	Total  int
	Ends   time.Time
	SentAt time.Time
}

func (c Cap) current(now time.Time) bool {
	return c.Status != "" && c.Status != NoCap && !now.After(c.Ends)
}

func (c Cap) Reached(now time.Time) bool {
	return c.Status == Capped && c.current(now)
}

func (c Cap) Warned(now time.Time) bool {
	return (c.Status == FirstWarning || c.Status == SecondWarning) && c.current(now)
}

type Notice struct {
	Timelock *Timelock
	Cap      *Cap
}

type number int64

func (n *number) UnmarshalJSON(raw []byte) error {
	text := string(bytes.Trim(raw, `"`))
	if text == "null" || text == "" {
		*n = 0
		return nil
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: number %s", ErrMalformed, raw)
	}
	*n = number(parsed)
	return nil
}

func (n number) time() time.Time {
	if n <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(n), 0)
}

type timelockData struct {
	Update *struct {
		Active bool    `json:"is_active"`
		Ends   number  `json:"time_enforcement_ends"`
		Kind   *string `json:"enforcement_type"`
	} `json:"xwa2_notify_account_reachout_timelock"`
}

type capData struct {
	Update *struct {
		Status string `json:"capping_status"`
		Used   number `json:"used_quota"`
		Total  number `json:"total_quota"`
		Ends   number `json:"cycle_end_timestamp"`
		SentAt number `json:"server_sent_timestamp"`
	} `json:"xwa2_notify_new_chat_messages_capping_info_update"`
}

func Parse(n node.Node, now time.Time) (Notice, error) {
	if kind, _ := n.Attr("type").Text(); n.Tag != "notification" || kind != "mex" {
		return Notice{}, fmt.Errorf("%w: not a mex notification", ErrMalformed)
	}
	update, ok := n.Child("update")
	if !ok {
		return Notice{}, fmt.Errorf("%w: no update", ErrMalformed)
	}
	op, _ := update.Attr("op_name").Text()
	switch op {
	case opTimelock:
		var data timelockData
		if err := decode(update.Bytes, &data); err != nil {
			return Notice{}, err
		}
		if data.Update == nil {
			return Notice{}, fmt.Errorf("%w: no timelock in %s", ErrMalformed, op)
		}
		t := Timelock{Active: data.Update.Active, Ends: data.Update.Ends.time(), Kind: "DEFAULT"}
		if data.Update.Kind != nil && *data.Update.Kind != "" {
			t.Kind = *data.Update.Kind
		}
		if t.Active && t.Ends.IsZero() {
			t.Ends = now.Add(defaultTimelock)
		}
		return Notice{Timelock: &t}, nil
	case opCap:
		var data capData
		if err := decode(update.Bytes, &data); err != nil {
			return Notice{}, err
		}
		if data.Update == nil {
			return Notice{}, fmt.Errorf("%w: no cap in %s", ErrMalformed, op)
		}
		u := data.Update
		c := Cap{Status: CapStatus(u.Status), Used: int(min(u.Used, u.Total)), Total: int(u.Total), Ends: u.Ends.time(), SentAt: u.SentAt.time()}
		if c.Status == "" {
			c.Status = NoCap
		}
		return Notice{Cap: &c}, nil
	}
	return Notice{}, nil
}

func decode(raw []byte, into any) error {
	var envelope struct {
		Data jsontext.Value `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("%w: no data", ErrMalformed)
	}
	if err := json.Unmarshal(envelope.Data, into); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return nil
}
