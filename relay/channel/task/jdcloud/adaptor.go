package jdcloud

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// ============================
// 灵境请求 / 响应结构
// ============================

// metaParams 从客户端 metadata 中提取的可选视频参数。
// 指针类型 + omitempty：缺省字段为 nil 并被忽略，显式 0/false 仍会透传。
type metaParams struct {
	Duration      *int    `json:"duration,omitempty"`
	Mode          *string `json:"mode,omitempty"`
	Resolution    *string `json:"resolution,omitempty"`
	Ratio         *string `json:"ratio,omitempty"`
	AspectRatio   *string `json:"aspect_ratio,omitempty"`
	GenerateAudio *bool   `json:"generate_audio,omitempty"`
	Tools         *bool   `json:"tools,omitempty"`      // 联网搜索(可选, 默认 false)
	ImageTail     *string `json:"image_tail,omitempty"` // 图生视频尾帧(可选)
}

type jdParams struct {
	Prompt        string `json:"prompt"`
	ModelName     string `json:"model_name"`
	Duration      string `json:"duration"`
	Mode          string `json:"mode"`
	AspectRatio   string `json:"aspect_ratio"`
	GenerateAudio bool   `json:"generate_audio"`
	Tools         bool   `json:"tools"`
	Image         string `json:"image,omitempty"`      // 图生视频首帧(图生视频必填)
	ImageTail     string `json:"image_tail,omitempty"` // 图生视频尾帧(可选)
}

type jdSubmitRequest struct {
	APIID  string   `json:"apiId"`
	Params jdParams `json:"params"`
}

type jdOuterError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// jdSubmitResponse 提交任务响应：result.result.genTaskId 为任务 ID。
type jdSubmitResponse struct {
	RequestID string        `json:"requestId"`
	Error     *jdOuterError `json:"error"`
	Result    struct {
		Result struct {
			GenTaskID string `json:"genTaskId"`
			Success   bool   `json:"success"`
			Err       string `json:"error"`
		} `json:"result"`
	} `json:"result"`
}

type jdTaskResult struct {
	URL          string `json:"url"`
	WatermarkURL string `json:"watermarkUrl"`
	ErrorReason  string `json:"errorReason"`
}

// jdTaskResultData 查询响应的内层业务对象 (result.result)。
type jdTaskResultData struct {
	TaskStatus  int            `json:"taskStatus"` // 0 任务中 / 1 完成 / 2 失败 / 4 水印完成(视频就绪)
	TaskResults []jdTaskResult `json:"taskResults"`
	ErrMsg      string         `json:"errMsg"`
	Success     bool           `json:"success"`
}

// jdQueryResponse 查询任务响应。
type jdQueryResponse struct {
	RequestID string        `json:"requestId"`
	Error     *jdOuterError `json:"error"`
	Result    struct {
		Result jdTaskResultData `json:"result"`
	} `json:"result"`
}

// ============================
// Adaptor 实现
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling // 提供 EstimateBilling / AdjustBillingOnSubmit / AdjustBillingOnComplete 的 no-op 默认实现
	ChannelType            int
	apiKey                 string
	baseURL                string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction 默认按文生视频(textGenerate)处理；若带图片则判为图生视频(generate)。
// 仅影响后台任务日志的分类标签，不影响灵境实际处理（灵境按 params 自行判断）。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate); taskErr != nil {
		return taskErr
	}
	if req, err := relaycommon.GetTaskRequest(c); err == nil && req.HasImage() {
		info.Action = constant.TaskActionGenerate
	}
	return nil
}

// BuildRequestURL 构造灵境提交任务地址。
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return strings.TrimRight(a.baseURL, "/") + submitPath, nil
}

// BuildRequestHeader 设置灵境所需的请求头，含每次不重复的 x-jdcloud-request-id。
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("x-jdcloud-request-id", uuid.NewString())
	return nil
}

// BuildRequestBody 将统一任务请求翻译成灵境 ydSubmitTask 的 {apiId, params} 结构。
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	var mp metaParams
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &mp); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// model_name：默认固定值；若渠道做了模型映射则用映射后的上游名。
	modelName := DefaultModel
	if info.IsModelMapped && info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	} else if req.Model != "" {
		modelName = req.Model
	}
	if !info.IsModelMapped {
		info.UpstreamModelName = modelName
	}

	tools := false // 联网搜索，默认关闭
	if mp.Tools != nil {
		tools = *mp.Tools
	}

	params := jdParams{
		Prompt:        req.Prompt,
		ModelName:     modelName,
		Duration:      resolveDuration(req, mp),
		Mode:          resolveMode(mp),
		AspectRatio:   resolveRatio(mp),
		GenerateAudio: resolveGenerateAudio(mp),
		Tools:         tools,
	}

	// 图生视频：带首帧图则走 apiId 753，并带上 image(必填)/image_tail(可选)；否则文生视频 apiId 752。
	apiID := DefaultAPIID
	if req.HasImage() {
		apiID = ImageAPIID
		params.Image = req.Images[0]
		if mp.ImageTail != nil && *mp.ImageTail != "" {
			params.ImageTail = *mp.ImageTail
		} else if len(req.Images) >= 2 {
			params.ImageTail = req.Images[1]
		}
	}

	body := jdSubmitRequest{APIID: apiID, Params: params}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest 委托通用 helper 发起上游请求。
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 解析灵境提交响应，取出 genTaskId 作为上游任务 ID 返回。
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var dResp jdSubmitResponse
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.Result.Result.GenTaskID == "" {
		taskErr = service.TaskErrorWrapper(errors.New(submitFailReason(dResp)), "invalid_response", http.StatusBadGateway)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)

	return dResp.Result.Result.GenTaskID, responseBody, nil
}

// FetchTask 轮询：POST 灵境 queryTasKResult。
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	payload, err := common.Marshal(map[string]string{"genTaskId": taskID})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseUrl, "/")+queryPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("x-jdcloud-request-id", uuid.NewString())

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ParseTaskResult 将灵境查询响应映射为内部 TaskInfo。
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp jdQueryResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal jdcloud task result failed")
	}

	inner := qResp.Result.Result
	taskResult := relaycommon.TaskInfo{Code: 0}

	// 灵境 taskStatus：0 任务中 / 1 完成 / 2 失败 / 4 水印完成(视频就绪)。
	// 文档只列了 0/1/2，但实际接口会返回 4，故以"是否已产出视频 url"作为成功判据最稳。
	if inner.TaskStatus == 2 {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = failReason(inner)
	} else if url := firstVideoURL(inner.TaskResults); url != "" {
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = url
	} else {
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	}

	return &taskResult, nil
}

// ConvertToOpenAIVideo 将存储的任务数据转换为 OpenAI 视频响应，视频地址放入 metadata.url。
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var qResp jdQueryResponse
	if err := common.Unmarshal(originTask.Data, &qResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal jdcloud task data failed")
	}
	inner := qResp.Result.Result

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", firstVideoURL(inner.TaskResults))
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if inner.TaskStatus == 2 {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: failReason(inner),
		}
	}

	return common.Marshal(openAIVideo)
}

// ============================
// 参数解析辅助
// ============================

func resolveDuration(req relaycommon.TaskSubmitReq, mp metaParams) string {
	if sec, err := strconv.Atoi(req.Seconds); err == nil && sec > 0 {
		return strconv.Itoa(sec)
	}
	if req.Duration > 0 {
		return strconv.Itoa(req.Duration)
	}
	if mp.Duration != nil && *mp.Duration > 0 {
		return strconv.Itoa(*mp.Duration)
	}
	return "5"
}

func resolveMode(mp metaParams) string {
	if mp.Mode != nil && *mp.Mode != "" {
		return strings.ToLower(*mp.Mode)
	}
	if mp.Resolution != nil && *mp.Resolution != "" {
		switch strings.ToLower(*mp.Resolution) {
		case "480p", "720p", "1080p":
			return strings.ToLower(*mp.Resolution)
		case "4k", "2160p":
			return "1080p"
		}
	}
	return "720p"
}

func resolveRatio(mp metaParams) string {
	if mp.AspectRatio != nil && *mp.AspectRatio != "" {
		return *mp.AspectRatio
	}
	if mp.Ratio != nil && *mp.Ratio != "" {
		return *mp.Ratio
	}
	return "16:9"
}

func resolveGenerateAudio(mp metaParams) bool {
	if mp.GenerateAudio != nil {
		return *mp.GenerateAudio
	}
	return true
}

func firstVideoURL(results []jdTaskResult) string {
	for _, r := range results {
		if r.URL != "" {
			return r.URL
		}
	}
	for _, r := range results {
		if r.WatermarkURL != "" {
			return r.WatermarkURL
		}
	}
	return ""
}

func failReason(inner jdTaskResultData) string {
	for _, r := range inner.TaskResults {
		if r.ErrorReason != "" {
			return r.ErrorReason
		}
	}
	if inner.ErrMsg != "" {
		return inner.ErrMsg
	}
	return "task failed"
}

func submitFailReason(dResp jdSubmitResponse) string {
	if dResp.Error != nil && dResp.Error.Message != "" {
		return dResp.Error.Message
	}
	if dResp.Result.Result.Err != "" {
		return dResp.Result.Result.Err
	}
	return "genTaskId is empty"
}
