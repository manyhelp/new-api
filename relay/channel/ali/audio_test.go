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
// CosyVoice synthesis request. dramaclaw's contract is verified from
// generators/indextts2_fal.py:_generate_via_newapi.

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
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3-plus"), req)
	require.NoError(t, err)
	assert.Equal(t, "cosyvoice-v3-plus", got.Model)
	assert.Equal(t, "你好世界", got.Input)
	assert.Equal(t, "https://example.com/voice.wav", got.ReferenceAudio)
	assert.Equal(t, "excited", got.EmotionPrompt)
	assert.Equal(t, defaultCosyVoiceVoice, got.Voice)
}

func TestBuildCosyVoiceRequestWithoutEmotion(t *testing.T) {
	meta := json.RawMessage(`{"audio_url":"https://example.com/v.wav"}`)
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: meta}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3-plus"), req)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v.wav", got.ReferenceAudio)
	assert.Empty(t, got.EmotionPrompt)
}

func TestBuildCosyVoiceRequestNoMetadataUsesDefaultVoice(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi"}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3-plus"), req)
	require.NoError(t, err)
	assert.Empty(t, got.ReferenceAudio)
	assert.Equal(t, defaultCosyVoiceVoice, got.Voice)
}

func TestBuildCosyVoiceRequestEmptyMetadataObject(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{}`)}
	got, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3-plus"), req)
	require.NoError(t, err)
	assert.Empty(t, got.ReferenceAudio)
	assert.Equal(t, defaultCosyVoiceVoice, got.Voice)
}

func TestBuildCosyVoiceRequestBadMetadataErrors(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{invalid`)}
	_, err := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3-plus"), req)
	require.Error(t, err)
}
