package ali

import (
	"encoding/json"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCosyVoiceRequest builds the DashScope synthesis body for a resolved
// voice_id: {model, input:{text, voice, format, sample_rate}}. Per the SDK
// HttpSpeechSynthesizer (voice inside input).

func newCosyVoiceInfo(upstream string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: upstream},
	}
}

func TestBuildCosyVoiceRequest(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "你好世界"}
	got := buildCosyVoiceRequest(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req, "voice-abc123")
	assert.Equal(t, "cosyvoice-v3.5-flash", got.Model)
	assert.Equal(t, "你好世界", got.Input.Text)
	assert.Equal(t, "voice-abc123", got.Input.Voice)
	assert.Equal(t, "mp3", got.Input.Format)
	assert.Equal(t, 24000, got.Input.SampleRate)
}

func TestCosyVoicePrefix(t *testing.T) {
	// deterministic + fits the DashScope rule (<10 chars, lowercase+digits).
	a := cosyVoicePrefix("https://example.com/a.wav")
	b := cosyVoicePrefix("https://example.com/a.wav")
	assert.Equal(t, a, b)
	assert.Equal(t, "dc"+a[2:], a) // starts with "dc"
	assert.LessOrEqual(t, len(a), 10)
	// different audio → different prefix
	c := cosyVoicePrefix("https://example.com/b.wav")
	assert.NotEqual(t, a, c)
}

func TestResolveCosyVoiceVoiceIDNoMetadataUsesDefault(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi"}
	voiceID, err := resolveCosyVoiceVoiceID(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Equal(t, defaultCosyVoiceVoice, voiceID)
}

func TestResolveCosyVoiceVoiceIDEmptyMetadataUsesDefault(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{}`)}
	voiceID, err := resolveCosyVoiceVoiceID(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Equal(t, defaultCosyVoiceVoice, voiceID)
}

func TestResolveCosyVoiceVoiceIDRequestVoiceWins(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Voice: "longxiaochun"}
	voiceID, err := resolveCosyVoiceVoiceID(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.NoError(t, err)
	assert.Equal(t, "longxiaochun", voiceID)
}

func TestResolveCosyVoiceVoiceIDBadMetadataErrors(t *testing.T) {
	req := dto.AudioRequest{Model: "index-tts-2", Input: "hi", Metadata: json.RawMessage(`{invalid`)}
	_, err := resolveCosyVoiceVoiceID(newCosyVoiceInfo("cosyvoice-v3.5-flash"), req)
	require.Error(t, err)
}

// NOTE: the metadata.audio_url path (registration + polling) does real HTTP to
// DashScope and is not unit-tested here; it needs a live DashScope account.
