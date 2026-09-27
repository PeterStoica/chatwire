package limits_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/limits"
	"github.com/PeterStoica/chatwire/internal/node"
)

func mex(op, payload string) node.Node {
	return node.Node{
		Tag:   "notification",
		Attrs: []node.Attr{{Key: "type", Value: node.Text("mex")}, {Key: "id", Value: node.Text("m1")}},
		Children: []node.Node{{
			Tag: "update", Attrs: []node.Attr{{Key: "op_name", Value: node.Text(op)}}, Bytes: []byte(payload),
		}},
	}
}

func TestReadingAccountLimits(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_900_000_000, 0)
	const timelock, capping = "NotificationUserReachoutTimelockUpdate", "MessageCappingInfoNotification"
	for _, tt := range []struct {
		name string
		n    node.Node
		want limits.Notice
	}{
		{
			name: "timelock until a time given as text",
			n:    mex(timelock, `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":true,"time_enforcement_ends":"1900003600","enforcement_type":"BULK_MESSAGING"}}}`),
			want: limits.Notice{Timelock: &limits.Timelock{Active: true, Ends: time.Unix(1_900_003_600, 0), Kind: "BULK_MESSAGING"}},
		},
		{
			name: "timelock until a time given as a number, no kind",
			n:    mex(timelock, `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":true,"time_enforcement_ends":1900000100,"enforcement_type":null}}}`),
			want: limits.Notice{Timelock: &limits.Timelock{Active: true, Ends: time.Unix(1_900_000_100, 0), Kind: "DEFAULT"}},
		},
		{
			name: "timelock without an end lasts a minute",
			n:    mex(timelock, `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":true}}}`),
			want: limits.Notice{Timelock: &limits.Timelock{Active: true, Ends: now.Add(time.Minute), Kind: "DEFAULT"}},
		},
		{
			name: "timelock lifted",
			n:    mex(timelock, `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":false}}}`),
			want: limits.Notice{Timelock: &limits.Timelock{Kind: "DEFAULT"}},
		},
		{
			name: "cap reached, used clamped to the quota",
			n: mex(capping, `{"data":{"xwa2_notify_new_chat_messages_capping_info_update":{"capping_status":"CAPPED","used_quota":12,"total_quota":10,`+
				`"cycle_start_timestamp":"1899000000","cycle_end_timestamp":"1901000000","server_sent_timestamp":"1900000000","ote_status":"ELIGIBLE"}}}`),
			want: limits.Notice{Cap: &limits.Cap{Status: limits.Capped, Used: 10, Total: 10, Ends: time.Unix(1_901_000_000, 0), SentAt: now}},
		},
		{
			name: "cap without a status",
			n:    mex(capping, `{"data":{"xwa2_notify_new_chat_messages_capping_info_update":{"used_quota":1,"total_quota":10}}}`),
			want: limits.Notice{Cap: &limits.Cap{Status: limits.NoCap, Used: 1, Total: 10}},
		},
		{
			name: "other operations are not limits",
			n:    mex("NotificationNewsletterJoin", `{"data":{}}`),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := limits.Parse(tt.n, now)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse() = %+v %+v, want %+v %+v", got.Timelock, got.Cap, tt.want.Timelock, tt.want.Cap)
			}
		})
	}
}

func TestMalformedLimitNotices(t *testing.T) {
	t.Parallel()
	for name, n := range map[string]node.Node{
		"not mex":          {Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("devices")}}},
		"no update":        {Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("mex")}}},
		"bad json":         mex("NotificationUserReachoutTimelockUpdate", `{"data":`),
		"null data":        mex("NotificationUserReachoutTimelockUpdate", `{"data":null,"errors":[{"message":"x"}]}`),
		"missing object":   mex("MessageCappingInfoNotification", `{"data":{}}`),
		"bad end":          mex("NotificationUserReachoutTimelockUpdate", `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":true,"time_enforcement_ends":"soon"}}}`),
		"missing timelock": mex("NotificationUserReachoutTimelockUpdate", `{"data":{"other":1}}`),
	} {
		if _, err := limits.Parse(n, time.Now()); !errors.Is(err, limits.ErrMalformed) {
			t.Errorf("%s: Parse() error = %v, want %v", name, err, limits.ErrMalformed)
		}
	}
}

func TestWhenALimitApplies(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_900_000_000, 0)
	lock := limits.Timelock{Active: true, Ends: now.Add(time.Hour)}
	if !lock.On(now) || lock.On(now.Add(time.Hour)) || (limits.Timelock{Ends: now.Add(time.Hour)}).On(now) {
		t.Fatal("a timelock applies only while active and before its end")
	}
	full := limits.Cap{Status: limits.Capped, Ends: now.Add(time.Hour)}
	if !full.Reached(now) || !full.Reached(now.Add(time.Hour)) || full.Reached(now.Add(time.Hour+time.Second)) || full.Warned(now) {
		t.Fatal("a reached cap applies until the end of its cycle")
	}
	warned := limits.Cap{Status: limits.SecondWarning, Ends: now.Add(time.Hour)}
	if !warned.Warned(now) || warned.Reached(now) || (limits.Cap{Status: limits.NoCap, Ends: now.Add(time.Hour)}).Warned(now) {
		t.Fatal("warnings are neither caps nor silence")
	}
}
