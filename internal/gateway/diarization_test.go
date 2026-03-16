package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSeg(speaker string, startMs, endMs int64, text string) TranscriptSegment {
	return TranscriptSegment{Speaker: speaker, StartMs: startMs, EndMs: endMs, Text: text}
}

func TestNewSpeakerMap_Empty(t *testing.T) {
	m := NewSpeakerMap(nil)
	id := m.Resolve("Speaker 1")
	assert.Equal(t, "Speaker 1", id.Label)
	assert.Equal(t, "Speaker 1", id.DisplayName)
}

func TestNewSpeakerMap_TwoParticipants(t *testing.T) {
	m := NewSpeakerMap([]SpeakerIdentity{{DisplayName: "Alice"}, {DisplayName: "Bob"}})
	assert.Equal(t, "Alice", m["Speaker 1"].DisplayName)
	assert.Equal(t, "Bob", m["Speaker 2"].DisplayName)
}

func TestBuildDiarizationResult_NilResult(t *testing.T) {
	r := BuildDiarizationResult(nil, nil)
	assert.Empty(t, r.Turns)
}

func TestBuildDiarizationResult_NoSpeakerLabel(t *testing.T) {
	result := &TranscriptResult{Segments: []TranscriptSegment{{Text: "hello"}}}
	r := BuildDiarizationResult(result, nil)
	require.Len(t, r.Turns, 1)
	assert.Equal(t, "Speaker 1", r.Turns[0].Speaker.Label)
}

func TestBuildDiarizationResult_TwoSpeakersAlternating(t *testing.T) {
	segs := []TranscriptSegment{
		makeSeg("Speaker 1", 0, 1000, "hi"),
		makeSeg("Speaker 2", 1000, 2000, "hello"),
		makeSeg("Speaker 1", 2000, 3000, "how"),
	}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, nil)
	assert.Equal(t, 3, r.TotalTurns)
}

func TestBuildDiarizationResult_ConsecutiveMerged(t *testing.T) {
	segs := []TranscriptSegment{
		makeSeg("Speaker 1", 0, 500, "word1"),
		makeSeg("Speaker 1", 500, 1000, "word2"),
		makeSeg("Speaker 1", 1000, 1500, "word3"),
	}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, nil)
	assert.Equal(t, 1, r.TotalTurns)
}

func TestBuildDiarizationResult_TextJoined(t *testing.T) {
	segs := []TranscriptSegment{
		makeSeg("Speaker 1", 0, 500, "hello"),
		makeSeg("Speaker 1", 500, 1000, "world"),
		makeSeg("Speaker 1", 1000, 1500, "test"),
	}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, nil)
	assert.Equal(t, "hello world test", r.Turns[0].Text)
}

func TestDiarizationResult_ToTranscriptText_Labels(t *testing.T) {
	segs := []TranscriptSegment{
		makeSeg("Speaker 1", 0, 1000, "hello"),
		makeSeg("Speaker 2", 1000, 2000, "hi"),
	}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, nil)
	out := r.ToTranscriptText(false)
	assert.Contains(t, out, "Speaker 1: ")
	assert.Contains(t, out, "Speaker 2: ")
}

func TestDiarizationResult_ToTranscriptText_DisplayNames(t *testing.T) {
	m := NewSpeakerMap([]SpeakerIdentity{{DisplayName: "Alice"}})
	segs := []TranscriptSegment{makeSeg("Speaker 1", 0, 1000, "hello")}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, m)
	out := r.ToTranscriptText(true)
	assert.Contains(t, out, "Alice: ")
}

func TestDiarizationResult_SortedSpeakers(t *testing.T) {
	segs := []TranscriptSegment{
		makeSeg("Speaker 2", 0, 500, "a"),
		makeSeg("Speaker 1", 500, 1000, "b"),
	}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, nil)
	sorted := r.SortedSpeakers()
	assert.Equal(t, "Speaker 1", sorted[0].Label)
	assert.Equal(t, "Speaker 2", sorted[1].Label)
}

func TestBuildDiarizationResult_WithSpeakerMap(t *testing.T) {
	m := NewSpeakerMap([]SpeakerIdentity{{DisplayName: "Alice", UserID: "user-1"}})
	segs := []TranscriptSegment{makeSeg("Speaker 1", 0, 1000, "hello")}
	r := BuildDiarizationResult(&TranscriptResult{Segments: segs}, m)
	require.Len(t, r.Turns, 1)
	assert.Equal(t, "user-1", r.Turns[0].Speaker.UserID)
}
