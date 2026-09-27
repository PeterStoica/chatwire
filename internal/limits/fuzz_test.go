package limits_test

import (
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/limits"
)

func FuzzLimitNotices(f *testing.F) {
	f.Add("NotificationUserReachoutTimelockUpdate", `{"data":{"xwa2_notify_account_reachout_timelock":{"is_active":true,"time_enforcement_ends":"1900003600"}}}`)
	f.Add("MessageCappingInfoNotification", `{"data":{"xwa2_notify_new_chat_messages_capping_info_update":{"capping_status":"CAPPED","used_quota":"5","total_quota":3}}}`)
	f.Fuzz(func(t *testing.T, op, payload string) {
		notice, err := limits.Parse(mex(op, payload), time.Unix(1_900_000_000, 0))
		if err == nil && notice.Cap != nil && notice.Cap.Used > notice.Cap.Total && notice.Cap.Total >= 0 {
			t.Fatalf("used %d of %d", notice.Cap.Used, notice.Cap.Total)
		}
	})
}
