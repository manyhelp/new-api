package jdcloud

// 京东云灵境 Cinema2.0 视频生成渠道配置。
//
// 灵境 JoyCreator OpenAPI 采用自定义异步任务协议：
//   提交: POST {base}/joycreator/openApi/ydSubmitTask   -> 返回 result.result.genTaskId
//   查询: POST {base}/joycreator/openApi/queryTasKResult -> 返回 result.result.taskStatus / taskResults
// 与 OpenAI/Sora 视频格式不兼容，故在此适配器内做协议翻译。

var ModelList = []string{
	"cinema-generate-2.0",
}

var ChannelName = "jdcloud-lingjing"

const (
	// DefaultAPIID 灵境文生视频(Cinema2.0)固定 apiId，文档要求不可修改。
	DefaultAPIID = "752"
	// ImageAPIID 灵境图生视频(Cinema2.0)固定 apiId。
	ImageAPIID = "753"
	// DefaultModel 灵境文生视频固定模型名。
	DefaultModel = "cinema-generate-2.0"

	submitPath = "/joycreator/openApi/ydSubmitTask"
	queryPath  = "/joycreator/openApi/queryTasKResult" // 注意灵境接口大小写：queryTasKResult
)
