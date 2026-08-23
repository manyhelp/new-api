package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

// dramaclaw 的 _canonicalize_video_payload 把时长规范成 `duration`(int) 透传，
// 不带 legacy `seconds`。doubao adaptor 必须读 req.Duration，否则 r.Duration 为 nil，
// 经 `duration,omitempty` 被丢弃，火山 Seedance 2.5 走默认 -1 → [4,30] 智能选择，
// 导致"画布选 4 秒却出 15 秒"。
func TestConvertToRequestPayloadDurationFromIntField(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "seedance-2.5",
		Prompt:   "a cat yawning",
		Duration: 4,
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)
	require.NotNil(t, r.Duration, "duration must be propagated from req.Duration")

	body, err := common.Marshal(r)
	require.NoError(t, err)
	require.Contains(t, string(body), `"duration":4`)
}

// 兼容仍以 `seconds`(string) 发出的旧客户端。
func TestConvertToRequestPayloadDurationFromLegacySeconds(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:   "seedance-2.5",
		Prompt:  "a cat yawning",
		Seconds: "4",
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)
	require.NotNil(t, r.Duration)

	body, err := common.Marshal(r)
	require.NoError(t, err)
	require.Contains(t, string(body), `"duration":4`)
}

// 两者并存时优先 int `duration`（请求级，语义更明确），避免 legacy seconds 覆盖。
func TestConvertToRequestPayloadDurationPrefersIntField(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "seedance-2.5",
		Prompt:   "a cat yawning",
		Duration: 4,
		Seconds:  "7",
	}

	r, err := adaptor.convertToRequestPayload(&req)
	require.NoError(t, err)

	body, err := common.Marshal(r)
	require.NoError(t, err)
	require.Contains(t, string(body), `"duration":4`)
	require.NotContains(t, string(body), `"duration":7`)
}
