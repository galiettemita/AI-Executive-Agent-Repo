package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestACIStateTracker_FirstFrameAlwaysTrue(t *testing.T) {
	t.Parallel()
	tracker := NewACIStateTracker()
	frame := make([]byte, 5000)
	for i := range frame {
		frame[i] = byte(i % 256)
	}
	assert.True(t, tracker.HasMeaningfulChange(frame), "first frame should always return true")
}

func TestACIStateTracker_IdenticalFrameReturnsFalse(t *testing.T) {
	t.Parallel()
	tracker := NewACIStateTracker()
	frame := make([]byte, 5000)
	for i := range frame {
		frame[i] = byte(i % 256)
	}
	tracker.HasMeaningfulChange(frame) // first call
	assert.False(t, tracker.HasMeaningfulChange(frame), "identical frame should return false")
}

func TestACIStateTracker_DifferentFrameReturnsTrue(t *testing.T) {
	t.Parallel()
	tracker := NewACIStateTracker()
	frame1 := make([]byte, 5000)
	frame2 := make([]byte, 5000)
	for i := range frame1 {
		frame1[i] = byte(i % 256)
		frame2[i] = byte((i + 128) % 256)
	}
	tracker.HasMeaningfulChange(frame1) // first
	assert.True(t, tracker.HasMeaningfulChange(frame2), "significantly different frame should return true")
}

func TestACIStateTracker_EmptyBytesReturnsTrue(t *testing.T) {
	t.Parallel()
	tracker := NewACIStateTracker()
	assert.True(t, tracker.HasMeaningfulChange([]byte{}), "empty frame should return true")
}

func TestHammingDistanceHex(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, hammingDistanceHex("0000", "0000"))
	assert.Equal(t, 4, hammingDistanceHex("000f", "0000")) // f=1111, 0=0000 → 4 bits
	assert.Greater(t, hammingDistanceHex("abcd", "1234"), 0)
}
