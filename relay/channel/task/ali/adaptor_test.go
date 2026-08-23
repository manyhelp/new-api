package ali

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func testRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
}

func TestConvertToAliRequestWan27I2VBuildsMediaFromImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "wan2.7-i2v",
		Prompt:   "animate the first frame",
		Image:    "https://example.com/first.png",
		Size:     "720p",
		Duration: 10,
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "wan2.7-i2v", aliReq.Model)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.Equal(t, 10, aliReq.Parameters.Duration)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
	}, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"media"`)
	require.NotContains(t, string(body), `"img_url"`)
}

func TestConvertToAliRequestWan27I2VBuildsFirstAndLastFrameFromImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "interpolate between frames",
		Images: []string{
			"https://example.com/first.png",
			"https://example.com/last.png",
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VPrefersImageBeforeImagesAndInputReference(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:          "wan2.7-i2v",
		Prompt:         "use the direct image",
		Image:          " https://example.com/direct.png ",
		Images:         []string{"https://example.com/images-first.png", " https://example.com/images-last.png "},
		InputReference: "https://example.com/input-reference.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/direct.png"},
		{Type: "last_frame", URL: "https://example.com/images-last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VFallsBackToFirstNonEmptyImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "skip blank images",
		Image:  " ",
		Images: []string{
			" ",
			" https://example.com/first.png ",
			" https://example.com/last.png ",
		},
		InputReference: "https://example.com/input-reference.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
}

func TestConvertToAliRequestWan27I2VKeepsExplicitMetadataMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:          "wan2.7-i2v",
		Prompt:         "continue the clip",
		Image:          "https://example.com/direct.png",
		Images:         []string{"https://example.com/images-first.png", "https://example.com/images-last.png"},
		InputReference: "https://example.com/input-reference.png",
		Metadata: map[string]interface{}{
			"input": map[string]interface{}{
				"media": []interface{}{
					map[string]interface{}{
						"type": "first_clip",
						"url":  "https://example.com/input.mp4",
					},
				},
			},
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_clip", URL: "https://example.com/input.mp4"},
	}, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"media"`)
	require.NotContains(t, string(body), `"img_url"`)
}

func TestConvertToAliRequestWan27I2VRequiresMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.7-i2v",
		Prompt: "animate without a frame",
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "requires image"))
}

func TestConvertToAliRequestWan25I2VKeepsLegacyImgURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan2.5-i2v-preview",
		Prompt: "animate the first frame",
		Image:  "https://example.com/first.png",
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "https://example.com/first.png", aliReq.Input.ImgURL)
	require.Empty(t, aliReq.Input.Media)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"img_url"`)
	require.NotContains(t, string(body), `"media"`)
}

// dramaclaw 首尾帧模式经 relay 层归一化后的形态：
// Image=首帧、Images=[首帧,尾帧]、metadata.last_frame_image 仍在。
func TestConvertToAliRequestWan30VideoBuildsFirstAndLastFrame(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "interpolate between frames",
		Image:  "https://example.com/first.png",
		Images: []string{"https://example.com/first.png", "https://example.com/last.png"},
		Metadata: map[string]interface{}{
			"last_frame_image": "https://example.com/last.png",
			"resolution":       "720p",
			"ratio":            "16:9",
			"watermark":        false,
			"generate_audio":   false,
		},
		Duration: 10,
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.Equal(t, "16:9", aliReq.Parameters.Ratio)
	require.Equal(t, 10, aliReq.Parameters.Duration)
	require.NotNil(t, aliReq.Parameters.Audio)
	require.False(t, *aliReq.Parameters.Audio)
	require.Empty(t, aliReq.Input.ImgURL)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"audio":false`)
	require.NotContains(t, string(body), `"size"`)
	require.NotContains(t, string(body), `"prompt_extend"`)
	require.NotContains(t, string(body), `"img_url"`)
}

// 仅尾帧：协议层归一化后 Image 与 Images[0] 同为尾帧 URL，需按尾帧语义还原。
func TestConvertToAliRequestWan30VideoRestoresLastFrameOnly(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "end on this frame",
		Image:  "https://example.com/last.png",
		Images: []string{"https://example.com/last.png"},
		Metadata: map[string]interface{}{
			"last_frame_image": "https://example.com/last.png",
			"resolution":       "480p",
		},
		Duration: 5,
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "last_frame", URL: "https://example.com/last.png"},
	}, aliReq.Input.Media)
	require.Equal(t, "480P", aliReq.Parameters.Resolution)
}

// 全能参考模式：reference_images / videos / audios 按序进 media。
func TestConvertToAliRequestWan30VideoBuildsReferenceMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "make it consistent with references",
		Image:  "https://example.com/ref1.png",
		Images: []string{"https://example.com/ref1.png", "https://example.com/ref2.png"},
		Metadata: map[string]interface{}{
			"reference_images": []interface{}{"https://example.com/ref1.png", "https://example.com/ref2.png"},
			"reference_videos": []interface{}{"https://example.com/ref.mp4"},
			"reference_audios": []interface{}{"https://example.com/ref.mp3"},
			"aspect_ratio":     "9:16",
			"resolution":       "1080p",
		},
		Duration: 5,
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, []AliVideoMedia{
		{Type: "reference_image", URL: "https://example.com/ref1.png"},
		{Type: "reference_image", URL: "https://example.com/ref2.png"},
		{Type: "reference_video", URL: "https://example.com/ref.mp4"},
		{Type: "reference_audio", URL: "https://example.com/ref.mp3"},
	}, aliReq.Input.Media)
	require.Equal(t, "1080P", aliReq.Parameters.Resolution)
	require.Equal(t, "9:16", aliReq.Parameters.Ratio)
}

// 文生视频：无 media，分辨率/比例/声音落默认档（720P / adaptive / 无声）。
func TestConvertToAliRequestWan30VideoTextToVideoDefaults(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "wan3.0-video",
		Prompt:   "a cinematic shot",
		Duration: 5,
		Metadata: map[string]interface{}{
			"resolution":     "720p",
			"ratio":          "adaptive",
			"watermark":      false,
			"generate_audio": false,
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Empty(t, aliReq.Input.Media)
	require.Empty(t, aliReq.Input.ImgURL)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.Equal(t, "adaptive", aliReq.Parameters.Ratio)
	require.NotNil(t, aliReq.Parameters.Audio)
	require.False(t, *aliReq.Parameters.Audio)
}

func TestConvertToAliRequestWan30VideoRejectsDurationOutOfRange(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "wan3.0-video",
		Prompt:   "too long",
		Duration: 31,
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "2-30"))
}

func TestConvertToAliRequestWan30VideoRejectsFramesPlusReferences(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:  "wan3.0-video",
		Prompt: "mixed media",
		Images: []string{"https://example.com/first.png", "https://example.com/ref.png"},
		Metadata: map[string]interface{}{
			"last_frame_image": "https://example.com/last.png",
			"reference_images": []interface{}{"https://example.com/ref.png"},
		},
		Duration: 5,
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "mutually exclusive"))
}

func TestConvertToAliRequestWan30VideoRejectsTooManyReferenceImages(t *testing.T) {
	tooMany := make([]interface{}, 11)
	for i := range tooMany {
		tooMany[i] = "https://example.com/ref.png"
	}
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model: "wan3.0-video",
		Metadata: map[string]interface{}{
			"reference_images": tooMany,
		},
		Duration: 5,
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "at most 10 reference images"))
}

func TestProcessAliOtherRatiosWan30Video(t *testing.T) {
	aliReq := &AliVideoRequest{
		Model: "wan3.0-video",
		Parameters: &AliVideoParameters{
			Resolution: "720P",
			Duration:   10,
		},
	}

	ratios, err := ProcessAliOtherRatios(aliReq)

	require.NoError(t, err)
	require.Equal(t, 2.0, ratios["resolution-720P"])
}

// HappyHorse：dramaclaw 文生视频形态（裸名 + metadata 分辨率/比例/水印）。
func TestConvertToAliRequestHappyHorseTextToVideoAppendsSuffix(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "a cinematic shot",
		Duration: 5,
		Metadata: map[string]interface{}{
			"resolution": "720P",
			"ratio":      "16:9",
			"watermark":  false,
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.0-t2v", aliReq.Model)
	require.Empty(t, aliReq.Input.Media)
	require.Equal(t, "720P", aliReq.Parameters.Resolution)
	require.Equal(t, "16:9", aliReq.Parameters.Ratio)
	require.Equal(t, 5, aliReq.Parameters.Duration)
	require.NotNil(t, aliReq.Parameters.Watermark)
	require.False(t, *aliReq.Parameters.Watermark)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.Contains(t, string(body), `"watermark":false`)
	require.NotContains(t, string(body), `"prompt_extend"`)
	require.NotContains(t, string(body), `"size"`)
	require.NotContains(t, string(body), `"img_url"`)
}

// HappyHorse：dramaclaw 首帧图生视频形态（metadata.image_url + Images 归一化后单图）。
// i2v 不接受 ratio（画幅跟随首帧），显式给出也必须丢弃。
func TestConvertToAliRequestHappyHorseFirstFrameDropsRatio(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1",
		Prompt:   "a cat running",
		Duration: 8,
		Image:    "https://example.com/first.png",
		Images:   []string{"https://example.com/first.png"},
		Metadata: map[string]interface{}{
			"image_url":  "https://example.com/first.png",
			"resolution": "1080P",
			"ratio":      "9:16",
			"watermark":  false,
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.1-i2v", aliReq.Model)
	require.Equal(t, []AliVideoMedia{
		{Type: "first_frame", URL: "https://example.com/first.png"},
	}, aliReq.Input.Media)
	require.Equal(t, "1080P", aliReq.Parameters.Resolution)
	require.Empty(t, aliReq.Parameters.Ratio)

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"ratio"`)
}

// HappyHorse：dramaclaw 参考生视频形态（metadata.reference_images 1-9 张）。
func TestConvertToAliRequestHappyHorseReferenceImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	refs := []interface{}{
		"https://example.com/girl.png",
		"https://example.com/fan.png",
		"https://example.com/earrings.png",
	}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "[Image 1]中的女性展开[Image 2]中的折扇",
		Duration: 5,
		Images: []string{
			"https://example.com/girl.png",
			"https://example.com/fan.png",
			"https://example.com/earrings.png",
		},
		Metadata: map[string]interface{}{
			"reference_images": refs,
			"resolution":       "720P",
			"ratio":            "9:16",
			"watermark":        false,
		},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.0-r2v", aliReq.Model)
	require.Equal(t, []AliVideoMedia{
		{Type: "reference_image", URL: "https://example.com/girl.png"},
		{Type: "reference_image", URL: "https://example.com/fan.png"},
		{Type: "reference_image", URL: "https://example.com/earrings.png"},
	}, aliReq.Input.Media)
	require.Equal(t, "9:16", aliReq.Parameters.Ratio)
}

// 直连 API 无 metadata：单图视为首帧，多图视为参考。
func TestConvertToAliRequestHappyHorseInfersModeFromImages(t *testing.T) {
	adaptor := &TaskAdaptor{}

	single := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1",
		Prompt:   "single image",
		Duration: 5,
		Image:    "https://example.com/only.png",
	}
	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), single)
	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.1-i2v", aliReq.Model)
	require.Equal(t, []AliVideoMedia{{Type: "first_frame", URL: "https://example.com/only.png"}}, aliReq.Input.Media)

	multi := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1",
		Prompt:   "reference images",
		Duration: 5,
		Images:   []string{"https://example.com/a.png", "https://example.com/b.png"},
	}
	aliReq, err = adaptor.convertToAliRequest(testRelayInfo(), multi)
	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.1-r2v", aliReq.Model)
	require.Len(t, aliReq.Input.Media, 2)
	require.Equal(t, "reference_image", aliReq.Input.Media[0].Type)
}

// 显式后缀名固定模式，素材不匹配必须报错（裸名才做自动路由）。
func TestConvertToAliRequestHappyHorseExplicitSuffixMismatch(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0-t2v",
		Prompt:   "text only model with image",
		Duration: 5,
		Image:    "https://example.com/first.png",
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "does not match request media"))
}

// 显式后缀名 + 匹配素材：模型名保持原样不再追加。
func TestConvertToAliRequestHappyHorseExplicitSuffixKept(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1-r2v",
		Prompt:   "explicit mode",
		Duration: 5,
		Images:   []string{"https://example.com/ref.png"},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.1-r2v", aliReq.Model)
	require.Equal(t, []AliVideoMedia{{Type: "reference_image", URL: "https://example.com/ref.png"}}, aliReq.Input.Media)
}

func TestConvertToAliRequestHappyHorseRejectsDurationOutOfRange(t *testing.T) {
	adaptor := &TaskAdaptor{}

	short := relaycommon.TaskSubmitReq{Model: "happyhorse-1.0", Prompt: "too short", Duration: 2}
	_, err := adaptor.convertToAliRequest(testRelayInfo(), short)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "3-15"))

	long := relaycommon.TaskSubmitReq{Model: "happyhorse-1.0", Prompt: "too long", Duration: 16}
	_, err = adaptor.convertToAliRequest(testRelayInfo(), long)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "3-15"))
}

func TestConvertToAliRequestHappyHorseRejectsTooManyReferenceImages(t *testing.T) {
	tooMany := make([]interface{}, 10)
	images := make([]string, 10)
	for i := range tooMany {
		tooMany[i] = "https://example.com/ref.png"
		images[i] = "https://example.com/ref.png"
	}
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "too many refs",
		Duration: 5,
		Images:   images,
		Metadata: map[string]interface{}{"reference_images": tooMany},
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "at most 9 reference images"))
}

func TestConvertToAliRequestHappyHorseRejectsInvalidRatioAndResolution(t *testing.T) {
	adaptor := &TaskAdaptor{}

	badRatio := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "bad ratio",
		Duration: 5,
		Metadata: map[string]interface{}{"ratio": "21:10"},
	}
	_, err := adaptor.convertToAliRequest(testRelayInfo(), badRatio)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "ratio"))

	badResolution := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "bad resolution",
		Duration: 5,
		Size:     "4K",
	}
	_, err = adaptor.convertToAliRequest(testRelayInfo(), badResolution)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "resolution"))
}

// 尾帧 / 视频编辑 / 视频参考不在对接范围内，必须显式报错而非静默降级。
func TestConvertToAliRequestHappyHorseRejectsUnsupportedInputs(t *testing.T) {
	adaptor := &TaskAdaptor{}

	lastFrame := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "with last frame",
		Duration: 5,
		Image:    "https://example.com/first.png",
		Metadata: map[string]interface{}{"last_frame_image": "https://example.com/last.png"},
	}
	_, err := adaptor.convertToAliRequest(testRelayInfo(), lastFrame)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "last frame"))

	videoEdit := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "video edit",
		Duration: 5,
		Metadata: map[string]interface{}{"video_url": "https://example.com/input.mp4"},
	}
	_, err = adaptor.convertToAliRequest(testRelayInfo(), videoEdit)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "video edit"))

	videoRef := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "video reference",
		Duration: 5,
		Metadata: map[string]interface{}{
			"reference_images": []interface{}{"https://example.com/ref.png"},
			"reference_videos": []interface{}{"https://example.com/ref.mp4"},
		},
	}
	_, err = adaptor.convertToAliRequest(testRelayInfo(), videoRef)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "reference videos"))
}

// 首帧与参考图互斥；metadata 首帧模式下多出的顶层图同样拒绝。
func TestConvertToAliRequestHappyHorseRejectsMixedMedia(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "mixed media",
		Duration: 5,
		Images:   []string{"https://example.com/first.png", "https://example.com/ref.png"},
		Metadata: map[string]interface{}{
			"image_url":        "https://example.com/first.png",
			"reference_images": []interface{}{"https://example.com/ref.png"},
		},
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "mutually exclusive"))
}

// seed 越界（计费外的确定性参数，但仍是上游校验值）必须报错。
func TestConvertToAliRequestHappyHorseRejectsSeedOutOfRange(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "bad seed",
		Duration: 5,
		Metadata: map[string]interface{}{"seed": float64(2147483648)},
	}

	_, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "seed"))
}

// 无 metadata 时水印不传（上游默认 true），seed 合法值透传。
func TestConvertToAliRequestHappyHorseDefaults(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.1",
		Prompt:   "defaults",
		Duration: 5,
		Metadata: map[string]interface{}{"seed": float64(42)},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)

	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.1-t2v", aliReq.Model)
	require.Equal(t, 5, aliReq.Parameters.Duration)
	require.Equal(t, 42, aliReq.Parameters.Seed)
	require.Nil(t, aliReq.Parameters.Watermark)
	require.Empty(t, aliReq.Parameters.Resolution) // 未显式指定 → 不传，上游默认 1080P
	require.Empty(t, aliReq.Parameters.Ratio)      // 未显式指定 → 不传，上游默认 16:9

	body, err := common.Marshal(aliReq)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"watermark"`)
	require.NotContains(t, string(body), `"resolution"`)
	require.NotContains(t, string(body), `"ratio"`)
}

// EstimateBilling：happyhorse 只按时长计费（seconds），无分辨率倍率。
func TestEstimateBillingHappyHorseSecondsOnly(t *testing.T) {
	adaptor := &TaskAdaptor{}
	req := relaycommon.TaskSubmitReq{
		Model:    "happyhorse-1.0",
		Prompt:   "billing",
		Duration: 12,
		Metadata: map[string]interface{}{"resolution": "1080P"},
	}

	aliReq, err := adaptor.convertToAliRequest(testRelayInfo(), req)
	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.0-t2v", aliReq.Model)

	ratios, err := ProcessAliOtherRatios(aliReq)
	require.NoError(t, err)
	require.Empty(t, ratios)
}
