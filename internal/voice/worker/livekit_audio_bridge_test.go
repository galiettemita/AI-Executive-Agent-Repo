package worker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAudioSource struct {
	frames [][]byte
	idx    int
	rate   int
}

func (m *mockAudioSource) ReadFrame(_ context.Context) ([]byte, error) {
	if m.idx >= len(m.frames) {
		return nil, nil
	}
	f := m.frames[m.idx]
	m.idx++
	return f, nil
}
func (m *mockAudioSource) SampleRate() int {
	if m.rate == 0 {
		return 16000
	}
	return m.rate
}

func TestNewLiveKitAudioBridge_EmptySessionID(t *testing.T) {
	_, err := NewLiveKitAudioBridge(LiveKitAudioBridgeConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SessionID required")
}

func TestNewLiveKitAudioBridge_NilAudioSource(t *testing.T) {
	_, err := NewLiveKitAudioBridge(LiveKitAudioBridgeConfig{SessionID: "s1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AudioSource required")
}

func TestPCM16BytesToInt16_BasicConversion(t *testing.T) {
	// [0x01,0x00, 0x00,0x01] → [1, 256]
	s := pcm16BytesToInt16([]byte{0x01, 0x00, 0x00, 0x01})
	assert.Equal(t, []int16{1, 256}, s)
}

func TestPCM16BytesToInt16_OddLength(t *testing.T) {
	s := pcm16BytesToInt16([]byte{0x01, 0x00, 0xFF}) // odd — last byte dropped
	assert.Len(t, s, 1)
	assert.Equal(t, int16(1), s[0])
}

func TestFrameDurMs_16kHz(t *testing.T) {
	// 320 bytes = 160 samples = 10ms at 16 kHz
	assert.Equal(t, 10, frameDurMs(320, 16000))
}

func TestFrameDurMs_ZeroSampleRate(t *testing.T) {
	assert.Equal(t, 0, frameDurMs(320, 0))
}
