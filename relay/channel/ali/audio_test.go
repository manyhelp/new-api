package ali

import (
	"encoding/json"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCosyVoiceRequest translates a dramaclaw IndexTTS2 voice-cloning request
// (sent as OpenAI /v1/audio/speech with metadata.audio_url) into a DashScope
// CosyVoice {model, input:{text}, parameters:{voice, format, ...}} request.
// dramaclaw's contract is verified from generators/indextts2_fal.py:_generate_via_newapi.

func newCosyVoiceInfo(upstream string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: upstream},
	}
}

func TestBuildCosyVoiceRequestCloning(t *testing.T) {
	meta := json.RawMessage(`{"audio_url":"https://example.com/voice.wav","should_use_prompt_for_emotion":true,"emotion_prompt":"excited"}`)
	req := dto.AudioRequest{
		Model:    "index-tts-2",
		Input:    "你好世界",
		Metadata: meta,
	}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Equal(t, "cosyvoice-v3.5-flash", got.Model)
	assert.Equal(t, "你好世界", got.Input.Text)
	assert.Equal(t, "https://example.com/voice.wav", got.Parameters.ReferenceAudio)
	assert.Equal(t, "excited", got.Parameters.EmotionPrompt)
	assert.Equal(t, defaultCosyVoiceVoice, got.Parameters.Voice)
	assert.Equal(t, "mp3", got.Parameters.Format)
}

func TestBuildCosyVoiceRequestWithoutEmotion(t *testing.T) {
	meta := json.RawMessage(`{"audio_url":"https://example.com/v.wav"}`)
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: meta}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v.wav", got.Parameters.ReferenceAudio)
	assert.Empty(t, got.Parameters.EmotionPrompt)
}

func TestBuildCosyVoiceRequestNoMetadataUsesDefaultVoice(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi"}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Empty(t, got.Parameters.ReferenceAudio)
	assert.Equal(t, defaultCosyVoiceVoice, got.Parameters.Voice)
}

func TestBuildCosyVoiceRequestEmptyMetadataObject(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{}`)}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Empty(t, got.Parameters.ReferenceAudio)
	assert.Equal(t, defaultCosyVoiceVoice, got.Parameters.Voice)
}

func TestBuildCosyVoiceRequestBadMetadataErrors(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{invalid`)}
	_, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.Error(t, err)
}
