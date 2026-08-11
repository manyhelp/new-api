package ali

// CosyVoice TTS relay for the Ali (DashScope) channel.
//
// dramaclaw's production TTS is IndexTTS2 voice-cloning, sent to new-api as an
// OpenAI /v1/audio/speech request with a CUSTOM body:
//
//	{
//	  "model": "index-tts-2",
//	  "input": "<text to synthesize>",
//	  "metadata": {
//	    "audio_url": "<reference voice sample URL>",
//	    "should_use_prompt_for_emotion": true,
//	    "emotion_prompt": "..."            // optional
//	  }
//	}
//
// new-api's AudioRequest DTO already carries `metadata` (json.RawMessage), so
// the clone reference survives the DTO layer. This adaptor translates that body
// into a DashScope CosyVoice synthesis request and normalizes the response back
// to what dramaclaw expects (raw audio bytes, or a JSON {"audio": <url>} that
// dramaclaw fetches).
//
// dramaclaw's CosyVoiceTTSGenerator (generators/tts_generator.py) already calls
// cosyvoice-v3-plus via the dashscope SDK SpeechSynthesizer (WebSocket). That is
// the verified reference for model name + voice. The exact DashScope REST
// endpoint path and request/response field names below are derived from the SDK
// pattern and MUST be verified against the CosyVoice REST docs before going live:
//   - endpoint path (COSYVOICE_TTS_PATH, env-overridable)
//   - CosyVoiceTTSRequest json tags (reference_audio / emotion_prompt)
//   - response JSON paths probed in handleCosyVoiceTTSResponse
//
// See 方案书 §6 "CosyVoice TTS 适配器详细设计（方案 B）".

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// defaultCosyVoiceVoice matches dramaclaw's CosyVoiceTTSGenerator default
// (config cosyvoice_voice = "longxiaoxia_v3"). Used when dramaclaw does not send
// a `voice` field (the IndexTTS2 client never does — it sends metadata.audio_url
// for cloning instead).
const defaultCosyVoiceVoice = "longxiaoxia_v3"

// cosyVoiceMeta parses the metadata block dramaclaw's IndexTTS2 client sends.
type cosyVoiceMeta struct {
	AudioURL                  string `json:"audio_url"`
	ShouldUsePromptForEmotion bool   `json:"should_use_prompt_for_emotion"`
	EmotionPrompt             string `json:"emotion_prompt,omitempty"`
}

// CosyVoiceTTSRequest is the DashScope CosyVoice synthesis request.
//
// DashScope's standard envelope is {model, input, parameters} (the same shape
// the Ali video task adaptor uses). The model comes from info.UpstreamModelName
// (the new-api channel model_mapping maps dramaclaw's "index-tts-2" → the
// cosyvoice model, e.g. cosyvoice-v3.5-flash / cosyvoice-v3-plus).
//
// VERIFY against the cosyvoice-v3.5-flash REST docs (bailian console
// model-market detail page): the exact parameter field names + whether
// zero-shot voice cloning takes an inline reference audio or a pre-registered
// custom voice_id. reference_audio/emotion_prompt below are the inline-clone
// attempt; if v3.5-flash requires registration, the adaptor must register
// metadata.audio_url → voice_id (cached) before synthesis.
type CosyVoiceTTSRequest struct {
	Model      string              `json:"model"`
	Input      CosyVoiceInput      `json:"input"`
	Parameters CosyVoiceParameters `json:"parameters"`
}

type CosyVoiceInput struct {
	Text string `json:"text"`
}

type CosyVoiceParameters struct {
	Voice  string `json:"voice"`
	Format string `json:"format,omitempty"`
	// VERIFY: inline zero-shot clone inputs. If v3.5-flash requires a
	// registered voice_id instead, drop these and resolve voice_id from a
	// registration cache keyed by audio_url.
	ReferenceAudio string `json:"reference_audio,omitempty"`
	EmotionPrompt  string `json:"emotion_prompt,omitempty"`
}

// cosyVoiceTTSEndpoint returns the DashScope CosyVoice REST path.
// Overridable via COSYVOICE_TTS_PATH so the verified path can be set without a
// rebuild. Default follows the DashScope services/<category>/<task>/<action>
// convention — VERIFY before live.
func cosyVoiceTTSEndpoint() string {
	return common.GetEnvOrDefaultString("COSYVOICE_TTS_PATH", "/api/v1/services/audio/tts/generation")
}

// buildCosyVoiceRequest translates a dramaclaw IndexTTS2 /v1/audio/speech body
// into a DashScope CosyVoice request. Pure (no I/O) so it is unit-testable.
//
//   - model: uses info.UpstreamModelName (the channel model_mapping already
//     mapped dramaclaw's "index-tts-2" → the cosyvoice model).
//   - input.text: the text to synthesize.
//   - parameters.voice: request.Voice if provided, else defaultCosyVoiceVoice.
//   - parameters.reference_audio / emotion_prompt: from metadata when present.
func buildCosyVoiceRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (CosyVoiceTTSRequest, error) {
	out := CosyVoiceTTSRequest{
		Model: info.UpstreamModelName,
		Input: CosyVoiceInput{Text: request.Input},
		Parameters: CosyVoiceParameters{
			Voice:  defaultCosyVoiceVoice,
			Format: "mp3",
		},
	}
	if request.Voice != "" {
		out.Parameters.Voice = request.Voice
	}
	if len(request.Metadata) > 0 {
		var meta cosyVoiceMeta
		if err := json.Unmarshal(request.Metadata, &meta); err != nil {
			return out, fmt.Errorf("error unmarshalling cosyvoice metadata: %w", err)
		}
		if meta.AudioURL != "" {
			out.Parameters.ReferenceAudio = meta.AudioURL
		}
		if meta.EmotionPrompt != "" {
			out.Parameters.EmotionPrompt = meta.EmotionPrompt
		}
	}
	return out, nil
}

// handleCosyVoiceTTSResponse normalizes a DashScope CosyVoice response into the
// shape dramaclaw's IndexTTS2 client accepts: raw audio bytes (written directly)
// or a JSON {"audio": <url>} that dramaclaw fetches.
//
// dramaclaw's _generate_via_newapi handles both: non-JSON content-type → write
// response.content; JSON → read payload["audio"] (string URL or {"url":...}).
// We therefore pass raw bytes through, and for JSON responses extract an audio
// URL (redirect, which dramaclaw follows) or base64/hex data (decode).
//
// VERIFY: the probed JSON paths match DashScope's actual envelope; adjust after
// confirming against the CosyVoice REST docs.
func handleCosyVoiceTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read cosyvoice response: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		ct := contentType
		if ct == "" {
			ct = "audio/mpeg"
		}
		c.Data(http.StatusOK, ct, body)
		return &dto.Usage{PromptTokens: info.GetEstimatePromptTokens()}, nil
	}

	// JSON: probe common audio-URL locations in DashScope/CosyVoice envelopes.
	audioField := gjson.GetBytes(body, "audio")
	if !audioField.Exists() {
		audioField = gjson.GetBytes(body, "output.audio")
	}
	if !audioField.Exists() {
		audioField = gjson.GetBytes(body, "data.audio")
	}
	if !audioField.Exists() {
		if msg := gjson.GetBytes(body, "message").String(); msg != "" {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("cosyvoice upstream error: %s", msg),
				types.ErrorCodeBadResponse,
				http.StatusBadGateway,
			)
		}
		return nil, types.NewErrorWithStatusCode(
			errors.New("cosyvoice response missing audio data"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	// audio field may be a URL string or {"url": "..."}.
	var audioVal string
	switch audioField.Type {
	case gjson.String:
		audioVal = audioField.String()
	default:
		audioVal = audioField.Get("url").String()
	}

	if strings.HasPrefix(audioVal, "http") {
		// dramaclaw's httpx client follows redirects (follow_redirects=True).
		c.Redirect(http.StatusFound, audioVal)
		return &dto.Usage{PromptTokens: info.GetEstimatePromptTokens()}, nil
	}
	if b, derr := base64.StdEncoding.DecodeString(audioVal); derr == nil && len(b) > 0 {
		c.Data(http.StatusOK, "audio/mpeg", b)
		return &dto.Usage{PromptTokens: info.GetEstimatePromptTokens()}, nil
	}
	if b, derr := hex.DecodeString(audioVal); derr == nil && len(b) > 0 {
		c.Data(http.StatusOK, "audio/mpeg", b)
		return &dto.Usage{PromptTokens: info.GetEstimatePromptTokens()}, nil
	}
	return nil, types.NewErrorWithStatusCode(
		errors.New("cosyvoice audio field is not a URL or decodable payload"),
		types.ErrorCodeBadResponse,
		http.StatusBadGateway,
	)
}
