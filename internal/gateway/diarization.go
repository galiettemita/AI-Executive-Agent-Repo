package gateway

import (
	"fmt"
	"sort"
	"strings"
)

// SpeakerIdentity holds resolved information about a detected speaker.
type SpeakerIdentity struct {
	Label       string // e.g. "Speaker 1"
	DisplayName string // resolved display name or Label if unknown
	UserID      string // workspace user ID or ""
}

// SpeakerMap maps speaker labels to workspace-specific identities.
type SpeakerMap map[string]SpeakerIdentity

// NewSpeakerMap creates a SpeakerMap from an ordered participant list.
func NewSpeakerMap(participants []SpeakerIdentity) SpeakerMap {
	m := make(SpeakerMap, len(participants))
	for i, p := range participants {
		label := fmt.Sprintf("Speaker %d", i+1)
		dn := p.DisplayName
		if strings.TrimSpace(dn) == "" {
			dn = label
		}
		m[label] = SpeakerIdentity{Label: label, DisplayName: dn, UserID: p.UserID}
	}
	return m
}

// Resolve returns the identity for a speaker label; falls back to unresolved identity.
func (m SpeakerMap) Resolve(label string) SpeakerIdentity {
	if id, ok := m[label]; ok {
		return id
	}
	return SpeakerIdentity{Label: label, DisplayName: label}
}

// SpeakerTurn is a contiguous block of speech from one speaker.
type SpeakerTurn struct {
	Speaker  SpeakerIdentity
	Text     string
	StartMs  int64
	EndMs    int64
	Segments []TranscriptSegment
}

// DiarizationResult groups transcript segments by speaker.
type DiarizationResult struct {
	Turns      []SpeakerTurn
	Speakers   []SpeakerIdentity
	TotalTurns int
}

// BuildDiarizationResult groups TranscriptResult segments into speaker turns.
// Consecutive segments from the same speaker are merged.
func BuildDiarizationResult(result *TranscriptResult, speakerMap SpeakerMap) *DiarizationResult {
	if result == nil || len(result.Segments) == 0 {
		return &DiarizationResult{Turns: []SpeakerTurn{}, Speakers: []SpeakerIdentity{}}
	}
	if speakerMap == nil {
		speakerMap = make(SpeakerMap)
	}

	var turns []SpeakerTurn

	for _, seg := range result.Segments {
		if seg.Speaker == "" {
			seg.Speaker = "Speaker 1"
		}
		identity := speakerMap.Resolve(seg.Speaker)

		if len(turns) == 0 || turns[len(turns)-1].Speaker.Label != seg.Speaker {
			turns = append(turns, SpeakerTurn{
				Speaker:  identity,
				StartMs:  seg.StartMs,
				EndMs:    seg.EndMs,
				Segments: []TranscriptSegment{seg},
			})
		} else {
			last := &turns[len(turns)-1]
			last.EndMs = seg.EndMs
			last.Segments = append(last.Segments, seg)
		}
	}

	// Build turn text from word segments.
	for i := range turns {
		words := make([]string, 0, len(turns[i].Segments))
		for _, seg := range turns[i].Segments {
			words = append(words, seg.Text)
		}
		turns[i].Text = strings.Join(words, " ")
	}

	// Collect unique speakers in order of first appearance.
	seenLabels := make(map[string]bool)
	var speakers []SpeakerIdentity
	for _, t := range turns {
		if !seenLabels[t.Speaker.Label] {
			seenLabels[t.Speaker.Label] = true
			speakers = append(speakers, t.Speaker)
		}
	}

	return &DiarizationResult{Turns: turns, Speakers: speakers, TotalTurns: len(turns)}
}

// ToTranscriptText formats the result as a labelled transcript string.
func (d *DiarizationResult) ToTranscriptText(useDisplayNames bool) string {
	var sb strings.Builder
	for _, turn := range d.Turns {
		name := turn.Speaker.Label
		if useDisplayNames && turn.Speaker.DisplayName != "" {
			name = turn.Speaker.DisplayName
		}
		sb.WriteString(name)
		sb.WriteString(": ")
		sb.WriteString(turn.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// SortedSpeakers returns speakers sorted by label.
func (d *DiarizationResult) SortedSpeakers() []SpeakerIdentity {
	sorted := make([]SpeakerIdentity, len(d.Speakers))
	copy(sorted, d.Speakers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Label < sorted[j].Label
	})
	return sorted
}
