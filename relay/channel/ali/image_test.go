package ali

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 异步图片任务执行失败（如 wanx 尺寸超限 InvalidParameter）时，错误必须以
// 4xx 返回而不是沿用创建响应的 200，否则客户端把 error 体当成功响应解析，
// 只能报出 "images response missing data" 这类无因错误。
func TestAliImageHandlerFailedAsyncTaskReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pollServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": {
				"task_id": "task-1",
				"task_status": "FAILED",
				"code": "InvalidParameter",
				"message": "Either width or height should be between 512 and 1440."
			},
			"request_id": "req-1"
		}`))
	}))
	defer pollServer.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: pollServer.URL,
			ApiKey:         "test-key",
		},
	}

	createResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(`{
			"output": {"task_id": "task-1", "task_status": "PENDING"},
			"request_id": "req-1"
		}`)),
	}

	apiErr, _ := aliImageHandler(&Adaptor{}, c, createResp, info)

	require.NotNil(t, apiErr, "failed async task must return an error")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Contains(t, apiErr.Error(), "between 512 and 1440")
}
