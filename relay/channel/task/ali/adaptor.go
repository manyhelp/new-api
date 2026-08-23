package ali

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// AliVideoRequest 阿里通义万相视频生成请求
type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

// AliVideoMedia describes Wan2.7 image-to-video media inputs.
type AliVideoMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// AliVideoInput 视频输入参数
type AliVideoInput struct {
	Prompt         string          `json:"prompt,omitempty"`          // 文本提示词
	ImgURL         string          `json:"img_url,omitempty"`         // 首帧图像URL或Base64（图生视频）
	FirstFrameURL  string          `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string          `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	AudioURL       string          `json:"audio_url,omitempty"`       // 音频URL（wan2.5支持）
	Media          []AliVideoMedia `json:"media,omitempty"`           // 媒体列表（wan2.7-i2v新协议）
	NegativePrompt string          `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string          `json:"template,omitempty"`        // 视频特效模板
}

// AliVideoParameters 视频参数
type AliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P（图生视频、首尾帧生视频）
	Size         string `json:"size,omitempty"`          // 尺寸: 如 "832*480"（文生视频）
	Duration     int    `json:"duration,omitempty"`      // 时长: 3-10秒
	PromptExtend bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    *bool  `json:"watermark,omitempty"`     // 是否添加水印（指针：wan 默认不传，happyhorse 上游默认 true，需显式表达 false）
	Audio        *bool  `json:"audio,omitempty"`         // 是否添加音频（wan2.5）
	Seed         int    `json:"seed,omitempty"`          // 随机数种子
	Ratio        string `json:"ratio,omitempty"`         // 宽高比: 16:9/9:16/1:1/4:3/3:4（wan2.7-r2v 参考生视频）
}

// AliVideoResponse 阿里通义万相响应
type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

// AliVideoOutput 输出信息
type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

// AliUsage 使用统计
type AliUsage struct {
	Duration   dto.IntValue `json:"duration,omitempty"`
	VideoCount dto.IntValue `json:"video_count,omitempty"`
	SR         dto.IntValue `json:"SR,omitempty"`
}

type AliMetadata struct {
	// Input 相关
	AudioURL       string          `json:"audio_url,omitempty"`       // 音频URL
	ImgURL         string          `json:"img_url,omitempty"`         // 图片URL（图生视频）
	FirstFrameURL  string          `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string          `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	Media          []AliVideoMedia `json:"media,omitempty"`           // 媒体列表（wan2.7-i2v新协议）
	NegativePrompt string          `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string          `json:"template,omitempty"`        // 视频特效模板

	// Parameters 相关
	Resolution   *string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P
	Size         *string `json:"size,omitempty"`          // 尺寸: 如 "832*480"
	Duration     *int    `json:"duration,omitempty"`      // 时长
	PromptExtend *bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    *bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool   `json:"audio,omitempty"`         // 是否添加音频
	Seed         *int    `json:"seed,omitempty"`          // 随机数种子
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
	// ValidateMultipartDirect 负责解析并将原始 TaskSubmitReq 存入 context
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v1/services/aigc/video-generation/video-synthesis", a.baseURL), nil
}

// BuildRequestHeader sets required headers for Ali API
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable") // 阿里异步任务必须设置
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := common.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p = []string{
		"832*480",
		"480*832",
		"624*624",
	}
	size720p = []string{
		"1280*720",
		"720*1280",
		"960*960",
		"1088*832",
		"832*1088",
	}
	size1080p = []string{
		"1920*1080",
		"1080*1920",
		"1440*1440",
		"1632*1248",
		"1248*1632",
	}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	} else if lo.Contains(size720p, size) {
		return "720P", nil
	} else if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func ProcessAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	otherRatios := make(map[string]float64)
	aliRatios := map[string]map[string]float64{
		"wan3.0-video": {
			"480P":  1,
			"720P":  2,
			"1080P": 4,
		},
		"wan2.6-i2v": {
			"720P":  1,
			"1080P": 1 / 0.6,
		},
		"wan2.5-t2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-t2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.5-i2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-i2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.2-kf2v-flash": {
			"480P":  1,
			"720P":  2,
			"1080P": 4.8,
		},
		"wan2.2-i2v-flash": {
			"480P": 1,
			"720P": 2,
		},
		"wan2.2-s2v": {
			"480P": 1,
			"720P": 0.9 / 0.5,
		},
	}
	var resolution string

	// size match
	if aliReq.Parameters.Size != "" {
		toResolution, err := sizeToResolution(aliReq.Parameters.Size)
		if err != nil {
			return nil, err
		}
		resolution = toResolution
	} else {
		resolution = strings.ToUpper(aliReq.Parameters.Resolution)
		if !strings.HasSuffix(resolution, "P") {
			resolution = resolution + "P"
		}
	}
	if otherRatio, ok := aliRatios[aliReq.Model]; ok {
		if ratio, ok := otherRatio[resolution]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", resolution)] = ratio
		}
	}
	return otherRatios, nil
}

func isWan27I2VModel(model string) bool {
	return strings.HasPrefix(model, "wan2.7-i2v")
}

func isWan27R2VModel(model string) bool {
	return strings.HasPrefix(model, "wan2.7-r2v")
}

func isWan30VideoModel(model string) bool {
	return strings.HasPrefix(model, "wan3.0-video")
}

// wan3.0-video 官方参数档位。
var (
	wan30VideoResolutions = []string{"480P", "720P", "1080P"}
	wan30VideoRatios      = []string{"adaptive", "16:9", "4:3", "1:1", "3:4", "9:16"}
)

// metadataStringSlice 读取 metadata 扁平键里的字符串数组（dramaclaw 参考素材协议：
// reference_images / reference_videos / reference_audios）。
func metadataStringSlice(req relaycommon.TaskSubmitReq, key string) []string {
	raw, ok := req.Metadata[key].([]interface{})
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			values = append(values, strings.TrimSpace(s))
		}
	}
	return values
}

// metadataStringValue 依序返回首个非空字符串键（dramaclaw 的扁平 metadata 不会
// 被通用 unmarshal 映射进嵌套 input/parameters，必须手动读取）。
func metadataStringValue(req relaycommon.TaskSubmitReq, keys ...string) string {
	for _, key := range keys {
		if v, ok := req.Metadata[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeWan30ResolutionStr(value string) string {
	resolution := strings.ToUpper(strings.TrimSpace(value))
	if resolution != "" && !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	if lo.Contains(wan30VideoResolutions, resolution) {
		return resolution
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstTaskImage(req relaycommon.TaskSubmitReq) string {
	if image := strings.TrimSpace(req.Image); image != "" {
		return image
	}
	for _, image := range req.Images {
		if trimmed := strings.TrimSpace(image); trimmed != "" {
			return trimmed
		}
	}
	if inputReference := strings.TrimSpace(req.InputReference); inputReference != "" {
		return inputReference
	}
	return ""
}

func secondTaskImage(req relaycommon.TaskSubmitReq) string {
	nonEmptyImages := 0
	for _, image := range req.Images {
		trimmed := strings.TrimSpace(image)
		if trimmed == "" {
			continue
		}
		nonEmptyImages++
		if nonEmptyImages == 2 {
			return trimmed
		}
	}
	return ""
}

func normalizeWan27I2VInput(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	if !isWan27I2VModel(aliReq.Model) {
		return nil
	}

	if len(aliReq.Input.Media) == 0 {
		firstFrameURL := firstNonEmpty(aliReq.Input.FirstFrameURL, aliReq.Input.ImgURL, firstTaskImage(req))
		lastFrameURL := firstNonEmpty(aliReq.Input.LastFrameURL, secondTaskImage(req))
		audioURL := aliReq.Input.AudioURL

		if firstFrameURL != "" {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{
				Type: "first_frame",
				URL:  firstFrameURL,
			})
		}
		if lastFrameURL != "" {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{
				Type: "last_frame",
				URL:  lastFrameURL,
			})
		}
		if audioURL != "" {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{
				Type: "driving_audio",
				URL:  audioURL,
			})
		}
	}

	if len(aliReq.Input.Media) == 0 {
		return fmt.Errorf("wan2.7-i2v requires image, images, input_reference, or input.media")
	}

	// Wan2.7 image-to-video uses the new input.media protocol. Avoid sending
	// legacy fields that belong to wan2.6 and earlier image-to-video APIs.
	aliReq.Input.ImgURL = ""
	aliReq.Input.FirstFrameURL = ""
	aliReq.Input.LastFrameURL = ""
	aliReq.Input.AudioURL = ""
	return nil
}

// normalizeWan27R2VInput 把统一任务请求翻译成 wan2.7-r2v（参考生视频）协议。
//
// r2v 与 i2v 是两套素材语义（https://help.aliyun.com/zh/model-studio/wan-video-to-video-api-reference）：
//   - media type 只有 reference_image / reference_video / first_frame；
//   - 参考图 + 参考视频合计 ≤ 5，首帧最多 1 张；
//   - prompt 用"图1、图2""视频1"指代素材，顺序与 media 数组一致；
//   - 必须至少传 1 个参考素材（不支持纯文生）。
//
// 映射：顶层 Images 按序全部作为 reference_image（dramaclaw 图片参考/全能参考
// 模式的语义）；metadata.reference_videos 数组作为 reference_video。first_frame
// 不在此组装——dramaclaw 首帧/首尾帧模式应选 i2v 模型，r2v 只承接参考类模式。
func normalizeWan27R2VInput(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	if !isWan27R2VModel(aliReq.Model) {
		return nil
	}

	if len(aliReq.Input.Media) == 0 {
		for _, image := range req.Images {
			if trimmed := strings.TrimSpace(image); trimmed != "" {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{
					Type: "reference_image",
					URL:  trimmed,
				})
			}
		}
		if raw, ok := req.Metadata["reference_videos"].([]interface{}); ok {
			for _, item := range raw {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{
						Type: "reference_video",
						URL:  strings.TrimSpace(s),
					})
				}
			}
		}
	}

	total := len(aliReq.Input.Media)
	if total == 0 {
		return fmt.Errorf("wan2.7-r2v requires at least one reference image or video (images / metadata.reference_videos)")
	}
	if total > 5 {
		return fmt.Errorf("wan2.7-r2v accepts at most 5 reference media items, got %d", total)
	}

	// r2v 的宽高比走 parameters.ratio（与 i2v 用 resolution 不同）。
	// dramaclaw 把画布选的比例放 metadata.aspect_ratio / metadata.ratio。
	if aliReq.Parameters != nil && aliReq.Parameters.Ratio == "" {
		ratio := ""
		for _, key := range []string{"aspect_ratio", "ratio"} {
			if v, ok := req.Metadata[key].(string); ok && strings.TrimSpace(v) != "" {
				ratio = strings.TrimSpace(v)
				break
			}
		}
		if ratio != "" {
			aliReq.Parameters.Ratio = ratio
		}
	}

	// r2v 不消费 i2v 的 legacy 字段。
	aliReq.Input.ImgURL = ""
	aliReq.Input.FirstFrameURL = ""
	aliReq.Input.LastFrameURL = ""
	aliReq.Input.AudioURL = ""
	return nil
}

// normalizeWan30VideoInput 把统一任务请求翻译成 wan3.0-video（All-in-One）协议。
//
// wan3.0 与 wan2.7 同用 input.media，但素材语义更宽（官方
// https://help.aliyun.com/zh/model-studio/wan-video-generation-api-reference）：
//   - 文生/首帧/首尾帧/参考生视频合一，media type 有 first_frame / last_frame /
//     reference_image / reference_video / reference_audio；
//   - first_frame ≤1、last_frame ≤1，且与 reference_* 互斥；
//   - reference_image ≤10、reference_video ≤5、reference_audio ≤5；
//   - parameters 为 resolution(480P/720P/1080P) + ratio(adaptive/16:9/4:3/1:1/3:4/9:16)
//     + duration(2-30秒) + audio + seed + watermark，无 size / prompt_extend。
//
// 模式判定（dramaclaw 稳定媒体协议在 relay 层已把 last_frame_image /
// reference_images 追加进 req.Images，metadata 本身不动）：
//   - 参考模式：metadata.reference_* 任一非空且无首尾帧键；
//   - 首尾帧模式：metadata 首尾帧键，或顶层图（Image/Images 槽位约定：
//     [0]=首帧、[1]=尾帧；协议层归一化后仅尾帧时两者同为尾帧 URL，按尾帧还原）；
//   - 其余为纯文生视频（media 为空）。
func normalizeWan30VideoInput(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	if !isWan30VideoModel(aliReq.Model) {
		return nil
	}

	firstMeta := metadataStringValue(req, "first_frame_image", "image_url")
	lastMeta := metadataStringValue(req, "last_frame_image")
	refVideos := metadataStringSlice(req, "reference_videos")
	refAudios := metadataStringSlice(req, "reference_audios")
	refImages := metadataStringSlice(req, "reference_images")
	if len(refImages) == 0 && firstMeta == "" && lastMeta == "" && len(req.Images) > 0 {
		// 直连 API 不带 metadata 时，顶层 Images 整体视为参考图。
		refImages = req.Images
	}

	if len(aliReq.Input.Media) == 0 {
		switch {
		case len(refImages) > 0 || len(refVideos) > 0 || len(refAudios) > 0:
			if firstMeta != "" || lastMeta != "" {
				return fmt.Errorf("wan3.0-video: first/last frame and reference media are mutually exclusive")
			}
			if len(refImages) > 10 {
				return fmt.Errorf("wan3.0-video accepts at most 10 reference images, got %d", len(refImages))
			}
			if len(refVideos) > 5 {
				return fmt.Errorf("wan3.0-video accepts at most 5 reference videos, got %d", len(refVideos))
			}
			if len(refAudios) > 5 {
				return fmt.Errorf("wan3.0-video accepts at most 5 reference audios, got %d", len(refAudios))
			}
			for _, url := range refImages {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "reference_image", URL: url})
			}
			for _, url := range refVideos {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "reference_video", URL: url})
			}
			for _, url := range refAudios {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "reference_audio", URL: url})
			}
		default:
			first := firstNonEmpty(firstMeta, firstTaskImage(req))
			last := lastMeta
			if last == "" {
				if second := secondTaskImage(req); second != first {
					last = second
				}
			}
			if first != "" && first == last && firstMeta == "" {
				// 仅尾帧：协议层归一化后 req.Image 与 Images[0] 同为尾帧。
				first = ""
			}
			if first != "" {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "first_frame", URL: first})
			}
			if last != "" {
				aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "last_frame", URL: last})
			}
		}
	}

	// wan3.0 只走 input.media，清掉 legacy 图生视频字段。
	aliReq.Input.ImgURL = ""
	aliReq.Input.FirstFrameURL = ""
	aliReq.Input.LastFrameURL = ""
	aliReq.Input.AudioURL = ""

	// 分辨率：metadata.resolution（dramaclaw 传小写 "720p"）优先，其次通用转换
	// 已产出的 Parameters（顶层 size "720p"/"1280*720"），兜底 720P（官方默认
	// 1080P，我们的默认档位定 720P）。
	resolution := normalizeWan30ResolutionStr(metadataStringValue(req, "resolution"))
	if resolution == "" {
		resolution = normalizeWan30ResolutionStr(aliReq.Parameters.Resolution)
	}
	if resolution == "" && aliReq.Parameters.Size != "" {
		if mapped, err := sizeToResolution(aliReq.Parameters.Size); err == nil {
			resolution = mapped
		}
	}
	if resolution == "" {
		resolution = "720P"
	}
	aliReq.Parameters.Resolution = resolution
	aliReq.Parameters.Size = ""

	// 宽高比：metadata.ratio / aspect_ratio，默认 adaptive（官方默认）。
	ratio := metadataStringValue(req, "ratio", "aspect_ratio")
	if ratio == "" {
		ratio = strings.TrimSpace(aliReq.Parameters.Ratio)
	}
	if !lo.Contains(wan30VideoRatios, ratio) {
		ratio = "adaptive"
	}
	aliReq.Parameters.Ratio = ratio

	// 时长：官方区间 2-30 秒。
	if aliReq.Parameters.Duration < 2 || aliReq.Parameters.Duration > 30 {
		return fmt.Errorf("wan3.0-video duration must be within 2-30 seconds, got %d", aliReq.Parameters.Duration)
	}

	// 声音：默认无声（dramaclaw 流水线走 TTS 后配音；官方默认有声但同价）。
	audio := false
	if v, ok := req.Metadata["generate_audio"].(bool); ok {
		audio = v
	} else if v, ok := req.Metadata["audio"].(bool); ok {
		audio = v
	}
	aliReq.Parameters.Audio = &audio

	if v, ok := req.Metadata["watermark"].(bool); ok {
		aliReq.Parameters.Watermark = &v
	}

	// wan3.0 无 prompt_extend 参数。
	aliReq.Parameters.PromptExtend = false
	return nil
}

// HappyHorse 官方参数档位（阿里百炼 video-synthesis，三份 API 文档：
// 文生视频 / 图生视频-基于首帧 / 参考生视频）。
var (
	happyHorseResolutions = []string{"480P", "720P", "1080P"}
	happyHorseRatios      = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "4:5", "5:4", "9:21", "21:9"}
	happyHorseMaxRefImage = 9
	happyHorseMaxSeed     = 2147483647
)

// HappyHorse 三种上游模型后缀：文生 / 首帧图生 / 参考生。
// 上游模型名带模式后缀（happyhorse-1.1-t2v / -i2v / -r2v），而网关侧统一暴露
// 裸名（dramaclaw 目录只配 `happyhorse-1.0` 一个 ID），由请求素材推断模式后追加。
var happyHorseModeSuffixes = []string{"t2v", "i2v", "r2v"}

func isHappyHorseModel(model string) bool {
	return strings.HasPrefix(model, "happyhorse-")
}

// splitHappyHorseModel 拆出裸名与显式模式后缀（无后缀时 mode 为空）。
func splitHappyHorseModel(model string) (base, mode string) {
	for _, suffix := range happyHorseModeSuffixes {
		if strings.HasSuffix(model, "-"+suffix) {
			return strings.TrimSuffix(model, "-"+suffix), suffix
		}
	}
	return model, ""
}

// metadataIntValue 读取 metadata 扁平键里的整数值（JSON 反序列化的数字是
// float64；测试/直构路径可能是 int），仅接受非负整数。
func metadataIntValue(req relaycommon.TaskSubmitReq, key string) (int64, bool) {
	switch v := req.Metadata[key].(type) {
	case float64:
		if v == math.Trunc(v) && v >= 0 {
			return int64(v), true
		}
	case int:
		if v >= 0 {
			return int64(v), true
		}
	case int64:
		if v >= 0 {
			return v, true
		}
	}
	return 0, false
}

func normalizeHappyHorseResolution(value string) string {
	resolution := strings.ToUpper(strings.TrimSpace(value))
	if resolution != "" && !strings.HasSuffix(resolution, "P") {
		resolution += "P"
	}
	if lo.Contains(happyHorseResolutions, resolution) {
		return resolution
	}
	return ""
}

// normalizeHappyHorseInput 把统一任务请求翻译成 HappyHorse 协议并改写上游模型名。
//
// HappyHorse 与万相同走 DashScope 异步 video-synthesis，但模型按模式拆分
// （官方文档 https://help.aliyun.com/zh/model-studio/）：
//   - happyhorse-{ver}-t2v：纯文生，prompt 必填，无 media；
//   - happyhorse-{ver}-i2v：首帧图生，media 有且仅有 1 张 first_frame，
//     宽高比跟随首帧、不接受 ratio 参数；
//   - happyhorse-{ver}-r2v：参考生，media 为 1～9 张 reference_image，
//     prompt 用 [Image 1]/[Image 2] 按序指代。
//   - duration 取值 [3,15] 秒；resolution 480P/720P/1080P（上游默认 1080P）；
//     ratio 仅 t2v/r2v（上游默认 16:9）；watermark 上游默认 true；
//     seed ∈ [0, 2147483647]。
//
// 模式判定（dramaclaw 稳定媒体协议在 relay 层已把 metadata 首帧/参考图归一化
// 进 req.Images，metadata 本身不动）：
//   - metadata.image_url / first_frame_image 非空 → i2v；
//   - metadata.reference_images 非空 → r2v；
//   - 直连调用无 metadata 时：单图视为首帧（i2v），多图视为参考（r2v）；
//   - 无任何素材 → t2v。
//
// 显式后缀名（如 happyhorse-1.1-i2v）固定模式，素材不匹配时报错；
// 裸名按判定结果追加后缀。尾帧与视频编辑不在本对接范围内，显式报错。
func normalizeHappyHorseInput(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) error {
	if !isHappyHorseModel(aliReq.Model) {
		return nil
	}

	if metadataStringValue(req, "video_url") != "" {
		return fmt.Errorf("happyhorse does not support video edit input (video_url); only t2v / first_frame i2v / reference r2v are integrated")
	}
	if lastMeta := metadataStringValue(req, "last_frame_image"); lastMeta != "" {
		return fmt.Errorf("happyhorse does not support last frame input: %s", lastMeta)
	}
	// 通用稳定媒体协议里 happyhorse 只消费图片参考；视频/音频参考在此模型上
	// 没有对应能力，显式拒绝而不是静默丢弃素材。
	if len(metadataStringSlice(req, "reference_videos")) > 0 || len(metadataStringSlice(req, "reference_audios")) > 0 {
		return fmt.Errorf("happyhorse does not support reference videos or audios; only 1-%d reference images", happyHorseMaxRefImage)
	}

	firstMeta := metadataStringValue(req, "image_url", "first_frame_image")
	refImages := metadataStringSlice(req, "reference_images")
	// 顶层图列表（真实链路中 relay 层已把 Image 归一化进 Images，这里兜底单图字段）。
	images := req.Images
	if len(images) == 0 && strings.TrimSpace(req.Image) != "" {
		images = []string{strings.TrimSpace(req.Image)}
	}

	explicitMode := ""
	if _, mode := splitHappyHorseModel(aliReq.Model); mode != "" {
		explicitMode = mode
	}

	// 模式判定：metadata 首帧 → i2v；metadata 参考图 / 多图 → r2v；
	// 直连 API 单图默认视为首帧（显式 -r2v 时视为单张参考图）；无素材 → t2v。
	var mode string
	switch {
	case firstMeta != "":
		if len(refImages) > 0 {
			return fmt.Errorf("happyhorse first frame and reference images are mutually exclusive")
		}
		if len(images) > 1 {
			return fmt.Errorf("happyhorse i2v accepts exactly one first frame image, got %d images", len(images))
		}
		mode = "i2v"
	case len(refImages) > 0:
		mode = "r2v"
	case len(images) > 1:
		mode = "r2v"
		refImages = images
	case len(images) == 1 && explicitMode != "r2v":
		mode = "i2v"
	case len(images) == 1:
		mode = "r2v"
		refImages = images
	default:
		mode = "t2v"
	}

	if explicitMode != "" {
		if explicitMode != mode {
			return fmt.Errorf("happyhorse model %s does not match request media (detected %s); use the bare model name to auto-select the mode", aliReq.Model, mode)
		}
	} else {
		aliReq.Model = aliReq.Model + "-" + mode
	}

	switch mode {
	case "i2v":
		firstFrame := firstMeta
		if firstFrame == "" {
			firstFrame = images[0]
		}
		aliReq.Input.Media = []AliVideoMedia{{Type: "first_frame", URL: firstFrame}}
	case "r2v":
		if len(refImages) > happyHorseMaxRefImage {
			return fmt.Errorf("happyhorse accepts at most %d reference images, got %d", happyHorseMaxRefImage, len(refImages))
		}
		for _, url := range refImages {
			aliReq.Input.Media = append(aliReq.Input.Media, AliVideoMedia{Type: "reference_image", URL: url})
		}
	}

	// HappyHorse 只接受 input.prompt + input.media，清掉万相 legacy 字段。
	aliReq.Input.ImgURL = ""
	aliReq.Input.FirstFrameURL = ""
	aliReq.Input.LastFrameURL = ""
	aliReq.Input.AudioURL = ""
	aliReq.Input.NegativePrompt = ""
	aliReq.Input.Template = ""

	// 时长：官方区间 3-15 秒。
	if aliReq.Parameters.Duration < 3 || aliReq.Parameters.Duration > 15 {
		return fmt.Errorf("happyhorse duration must be within 3-15 seconds, got %d", aliReq.Parameters.Duration)
	}

	// 分辨率：metadata.resolution（dramaclaw 传 "720P"/"1080P"）优先；
	// 其次通用转换已产出的 Parameters（客户端 size "720p"/"1080P"）；
	// 都没有则不传（上游默认 1080P）。显式但非法的值直接报错。
	resolution := normalizeHappyHorseResolution(metadataStringValue(req, "resolution"))
	if resolution == "" && strings.TrimSpace(req.Size) != "" {
		resolution = normalizeHappyHorseResolution(aliReq.Parameters.Resolution)
		if resolution == "" {
			return fmt.Errorf("happyhorse resolution must be one of 480P/720P/1080P, got %s", aliReq.Parameters.Resolution)
		}
	}
	aliReq.Parameters.Resolution = resolution

	// 宽高比：仅 t2v/r2v 生效，i2v 由首帧画幅决定，带了会被上游 INVALID_PARAMS 拒绝。
	aliReq.Parameters.Ratio = strings.TrimSpace(aliReq.Parameters.Ratio)
	if aliReq.Parameters.Ratio == "" {
		aliReq.Parameters.Ratio = metadataStringValue(req, "ratio", "aspect_ratio")
	}
	if aliReq.Parameters.Ratio != "" {
		if mode == "i2v" {
			// 画幅跟随首帧，忽略而非透传，避免上游报错。
			aliReq.Parameters.Ratio = ""
		} else if !lo.Contains(happyHorseRatios, aliReq.Parameters.Ratio) {
			return fmt.Errorf("happyhorse ratio must be one of %s, got %s", strings.Join(happyHorseRatios, "/"), aliReq.Parameters.Ratio)
		}
	}

	// 水印：上游默认 true；仅当客户端显式给出时透传（指针表达显式 false）。
	if v, ok := req.Metadata["watermark"].(bool); ok {
		aliReq.Parameters.Watermark = &v
	}

	// seed：仅接受 metadata 扁平键或通用转换产物，范围 [0, 2147483647]。
	if seed, ok := metadataIntValue(req, "seed"); ok {
		if seed > int64(happyHorseMaxSeed) {
			return fmt.Errorf("happyhorse seed must be within [0, %d], got %d", happyHorseMaxSeed, seed)
		}
		aliReq.Parameters.Seed = int(seed)
	} else if aliReq.Parameters.Seed < 0 || aliReq.Parameters.Seed > happyHorseMaxSeed {
		return fmt.Errorf("happyhorse seed must be within [0, %d], got %d", happyHorseMaxSeed, aliReq.Parameters.Seed)
	}

	// HappyHorse 无 size / prompt_extend / audio 参数。
	aliReq.Parameters.Size = ""
	aliReq.Parameters.PromptExtend = false
	aliReq.Parameters.Audio = nil
	return nil
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}
	aliReq := &AliVideoRequest{
		Model: upstreamModel,
		Input: AliVideoInput{
			Prompt: req.Prompt,
			ImgURL: firstTaskImage(req),
		},
		Parameters: &AliVideoParameters{
			PromptExtend: true, // 默认开启智能改写
		},
	}

	// 处理分辨率映射
	if req.Size != "" {
		// text to video size must be contained *
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			return nil, fmt.Errorf("invalid size: %s, example: %s", req.Size, "1920*1080")
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else {
			resolution := strings.ToUpper(req.Size)
			// 支持 480p, 720p, 1080p 或 480P, 720P, 1080P
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	} else {
		// 根据模型设置默认分辨率
		if strings.Contains(req.Model, "t2v") { // image to video
			if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Size = "1920*1080"
			} else if strings.HasPrefix(req.Model, "wan2.2") {
				aliReq.Parameters.Size = "1920*1080"
			} else {
				aliReq.Parameters.Size = "1280*720"
			}
		} else {
			if strings.HasPrefix(req.Model, "wan2.6") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-flash") {
				aliReq.Parameters.Resolution = "720P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-plus") {
				aliReq.Parameters.Resolution = "1080P"
			} else {
				aliReq.Parameters.Resolution = "720P"
			}
		}
	}

	// 处理时长
	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			return nil, errors.Wrap(err, "convert seconds to int failed")
		} else {
			aliReq.Parameters.Duration = seconds
		}
	}
	if aliReq.Parameters.Duration <= 0 {
		aliReq.Parameters.Duration = 5 // 默认5秒
	}

	// 从 metadata 中提取额外参数
	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			err = common.Unmarshal(metadataBytes, aliReq)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}

	if err := normalizeWan27I2VInput(aliReq, req); err != nil {
		return nil, err
	}

	if err := normalizeWan27R2VInput(aliReq, req); err != nil {
		return nil, err
	}

	if err := normalizeWan30VideoInput(aliReq, req); err != nil {
		return nil, err
	}

	if err := normalizeHappyHorseInput(aliReq, req); err != nil {
		return nil, err
	}

	return aliReq, nil
}

// EstimateBilling 根据用户请求参数计算 OtherRatios（时长、分辨率等）。
// 在 ValidateRequestAndSetAction 之后、价格计算之前调用。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil
	}

	// metadata can override Duration past standard request validation;
	// cap it because it is used as a billing multiplier.
	otherRatios := map[string]float64{
		"seconds": float64(min(aliReq.Parameters.Duration, relaycommon.MaxTaskDurationSeconds)),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	return otherRatios
}

// DoRequest delegates to common helper
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// 解析阿里响应
	var aliResp AliVideoResponse
	if err := common.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 检查错误
	if aliResp.Code != "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}

	if aliResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 转换为 OpenAI 格式响应
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()

	// 返回 OpenAI 格式
	c.JSON(http.StatusOK, openAIResp)

	return aliResp.Output.TaskID, responseBody, nil
}

// FetchTask 查询任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

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

// ParseTaskResult 解析任务结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// 状态映射
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		// 阿里直接返回视频URL，不需要额外的代理端点
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(task.Data, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal ali response failed")
	}

	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt

	// 设置视频URL（核心字段）
	openAIResp.SetMetadata("url", aliResp.Output.VideoURL)

	// 错误处理
	if aliResp.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Code,
			Message: aliResp.Message,
		}
	} else if aliResp.Output.Code != "" {
		openAIResp.Error = &dto.OpenAIVideoError{
			Code:    aliResp.Output.Code,
			Message: aliResp.Output.Message,
		}
	}

	return common.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}
