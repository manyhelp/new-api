package ali

// CosyVoice TTS relay for the Ali (DashScope) channel — voice CLONING path.
//
// dramaclaw's IndexTTS2 client sends /v1/audio/speech with metadata.audio_url
// (a reference voice sample). DashScope CosyVoice cloning is TWO-PHASE:
//
//  1. VoiceEnrollmentService.create_voice(url) → voice_id (async; poll
//     query_voice until output.status == "OK").
//  2. SpeechSynthesizer(model, voice=voice_id, text) → output.audio.url.
//
// The adaptor caches audio_url → voice_id so each character's voice is
// registered once. Grounded in the dashscope SDK source
// (.venv/.../dashscope/audio):
//   - synthesis: HttpSpeechSynthesizer → POST {base}/api/v1/services/audio/tts/SpeechSynthesizer
//     body {model, input:{text, voice, format, sample_rate}} → output.audio.url
//   - enrollment: VoiceEnrollmentService → POST {base}/api/v1/services/audio/tts/customization
//     body {model:"voice-enrollment", input:{action:"create_voice"|"query_voice", ...}}
//     create_voice → output.voice_id ; query_voice → output.status ("OK"/"UNDEPLOYED")
//
// SIDE EFFECTS / LATENCY (be aware):
//   - First synthesis per unique audio_url blocks for voice registration +
//     polling (seconds to minutes). Cached after.
//   - Registration creates a voice in the DashScope account (one per unique
//     audio_url). The enrollment direct HTTP client bypasses the channel
//     proxy setting; if the channel needs a proxy, set it at the system level.
//   - Set RELAY_TIMEOUT >= registration duration to avoid the gateway timing
//     out before the first voice is ready.
//
// dramaclaw's contract verified from generators/indextts2_fal.py:_generate_via_newapi.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const defaultCosyVoiceVoice = "longxiaoxia_v3" // preset fallback when no audio_url
const cosyVoiceEnrollmentModel = "voice-enrollment"

var (
	cosyVoicePollInterval    = 10 * time.Second
	cosyVoicePollMaxAttempts = 30
)

// cosyVoiceTTSEndpoint / cosyVoiceEnrollmentEndpoint return the DashScope REST
// paths (from the channel base root, e.g. https://<workspace>.cn-beijing.maas.aliyuncs.com).
// Overridable via env without a rebuild.
func cosyVoiceTTSEndpoint() string {
	return common.GetEnvOrDefaultString("COSYVOICE_TTS_PATH", "/api/v1/services/audio/tts/SpeechSynthesizer")
}

func cosyVoiceEnrollmentEndpoint() string {
	return common.GetEnvOrDefaultString("COSYVOICE_ENROLLMENT_PATH", "/api/v1/services/audio/tts/customization")
}

// cosyVoiceMeta parses the metadata block dramaclaw's IndexTTS2 client sends.
type cosyVoiceMeta struct {
	AudioURL                  string `json:"audio_url"`
	ShouldUsePromptForEmotion bool   `json:"should_use_prompt_for_emotion"`
	EmotionPrompt             string `json:"emotion_prompt,omitempty"`
}

// CosyVoiceTTSRequest is the DashScope CosyVoice SYNTHESIS body. Per the SDK
// HttpSpeechSynthesizer, voice/format/sample_rate live INSIDE input (not a
// separate parameters object).
type CosyVoiceTTSRequest struct {
	Model string         `json:"model"`
	Input CosyVoiceInput `json:"input"`
}

type CosyVoiceInput struct {
	Text       string `json:"text"`
	Voice      string `json:"voice"` // registered voice_id or preset
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// buildCosyVoiceRequest builds the synthesis body for a resolved voice_id.
// Pure (no I/O) so it is unit-testable.
func buildCosyVoiceRequest(info *relaycommon.RelayInfo, request dto.AudioRequest, voiceID string) CosyVoiceTTSRequest {
	return CosyVoiceTTSRequest{
		Model: info.UpstreamModelName,
		Input: CosyVoiceInput{
			Text:       request.Input,
			Voice:      voiceID,
			Format:     "mp3",
			SampleRate: 24000,
		},
	}
}

// resolveCosyVoiceVoiceID resolves the voice_id for a dramaclaw audio request:
// metadata.audio_url → registered voice_id (cloning, cached); else request.Voice
// or the default preset. Blocking on first registration per unique audio_url.
func resolveCosyVoiceVoiceID(info *relaycommon.RelayInfo, request dto.AudioRequest) (string, error) {
	voiceID := defaultCosyVoiceVoice
	if request.Voice != "" {
		voiceID = request.Voice
	}
	if len(request.Metadata) > 0 {
		var meta cosyVoiceMeta
		if err := json.Unmarshal(request.Metadata, &meta); err != nil {
			return "", fmt.Errorf("error unmarshalling cosyvoice metadata: %w", err)
		}
		if meta.AudioURL != "" {
			resolved, err := resolveCosyVoiceID(info, meta.AudioURL)
			if err != nil {
				return "", fmt.Errorf("error resolving cosyvoice voice_id: %w", err)
			}
			voiceID = resolved
		}
	}
	return voiceID, nil
}

// --- voice_id cache (audio_url → voice_id, ready) ---

type cosyVoiceCacheEntry struct {
	voiceID string
	ready   bool
}

var (
	cosyVoiceCache   = make(map[string]*cosyVoiceCacheEntry)
	cosyVoiceCacheMu sync.Mutex
)

// resolveCosyVoiceID returns a ready voice_id for audioURL, registering +
// polling if not cached. Blocking on first use per unique audio_url.
func resolveCosyVoiceID(info *relaycommon.RelayInfo, audioURL string) (string, error) {
	cosyVoiceCacheMu.Lock()
	if e, ok := cosyVoiceCache[audioURL]; ok && e.ready {
		cosyVoiceCacheMu.Unlock()
		return e.voiceID, nil
	}
	cosyVoiceCacheMu.Unlock()

	voiceID, err := cosyVoiceCreateVoice(info, audioURL)
	if err != nil {
		return "", fmt.Errorf("cosyvoice create_voice: %w", err)
	}
	if err := cosyVoicePollUntilReady(info, voiceID); err != nil {
		return "", fmt.Errorf("cosyvoice voice %s: %w", voiceID, err)
	}

	cosyVoiceCacheMu.Lock()
	cosyVoiceCache[audioURL] = &cosyVoiceCacheEntry{voiceID: voiceID, ready: true}
	cosyVoiceCacheMu.Unlock()
	return voiceID, nil
}

// cosyVoicePrefix derives a <10-char lowercase+digits prefix from audioURL.
func cosyVoicePrefix(audioURL string) string {
	sum := sha256.Sum256([]byte(audioURL))
	h := fmt.Sprintf("%x", sum) // 64-char lowercase hex
	if len(h) > 6 {
		h = h[:6]
	}
	return "dc" + h
}

// cosyVoiceCreateVoice POSTs action=create_voice → output.voice_id.
func cosyVoiceCreateVoice(info *relaycommon.RelayInfo, audioURL string) (string, error) {
	body := map[string]any{
		"model": cosyVoiceEnrollmentModel,
		"input": map[string]any{
			"action":       "create_voice",
			"target_model": info.UpstreamModelName,
			"prefix":       cosyVoicePrefix(audioURL),
			"url":          audioURL,
		},
	}
	resp, err := cosyVoiceEnrollmentHTTP(info, body)
	if err != nil {
		return "", err
	}
	voiceID := gjson.GetBytes(resp, "output.voice_id").String()
	if voiceID == "" {
		return "", fmt.Errorf("no voice_id in response: %s", truncate(string(resp), 300))
	}
	return voiceID, nil
}

// cosyVoicePollUntilReady POSTs action=query_voice until status == "OK".
func cosyVoicePollUntilReady(info *relaycommon.RelayInfo, voiceID string) error {
	for i := 0; i < cosyVoicePollMaxAttempts; i++ {
		body := map[string]any{
			"model": cosyVoiceEnrollmentModel,
			"input": map[string]any{
				"action":   "query_voice",
				"voice_id": voiceID,
			},
		}
		resp, err := cosyVoiceEnrollmentHTTP(info, body)
		if err == nil {
			status := gjson.GetBytes(resp, "output.status").String()
			switch status {
			case "OK":
				return nil
			case "UNDEPLOYED", "FAILED":
				return fmt.Errorf("registration failed (status=%s): %s", status, truncate(string(resp), 300))
			}
			// other statuses (processing) → keep polling
		}
		time.Sleep(cosyVoicePollInterval)
	}
	return fmt.Errorf("timed out after %d attempts", cosyVoicePollMaxAttempts)
}

// cosyVoiceEnrollmentHTTP POSTs a DashScope enrollment action and returns the
// raw response body. Uses a direct client (side-channel; bypasses channel
// proxy — see file header).
func cosyVoiceEnrollmentHTTP(info *relaycommon.RelayInfo, body map[string]any) ([]byte, error) {
	url := strings.TrimRight(info.ChannelBaseUrl, "/") + cosyVoiceEnrollmentEndpoint()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return respBody, fmt.Errorf("dashscope enrollment HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}
	return respBody, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// handleCosyVoiceTTSResponse normalizes the DashScope synthesis response into
// the shape dramaclaw's IndexTTS2 client expects. Non-streaming returns
// {output:{audio:{url,...}}}; we re-wrap as {"audio": <url>} so dramaclaw
// (which reads payload["audio"]) fetches it. Raw audio bytes pass through.
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

	// JSON: extract output.audio.url (SDK HttpSpeechSynthesizer non-streaming shape).
	audioURL := gjson.GetBytes(body, "output.audio.url").String()
	if audioURL == "" {
		audioURL = gjson.GetBytes(body, "audio.url").String()
	}
	if audioURL == "" {
		if msg := gjson.GetBytes(body, "message").String(); msg != "" {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("cosyvoice upstream error: %s", msg),
				types.ErrorCodeBadResponse,
				http.StatusBadGateway,
			)
		}
		return nil, types.NewErrorWithStatusCode(
			errors.New("cosyvoice response missing output.audio.url"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	// Re-wrap so dramaclaw's _extract_audio_url finds payload["audio"].
	c.JSON(http.StatusOK, gin.H{"audio": audioURL})
	return &dto.Usage{PromptTokens: info.GetEstimatePromptTokens()}, nil
}
