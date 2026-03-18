package vision

import (
	"context"
	"fmt"
	"strings"

	"github.com/brevio/brevio/internal/executor/connectors"
	"github.com/brevio/brevio/internal/gateway"
)

const frameBatchSize = 5

// SceneDescription describes a single scene from a video frame.
type SceneDescription struct {
	FrameIndex  int     `json:"frame_index"`
	Timestamp   float64 `json:"timestamp_seconds"`
	Description string  `json:"description"`
}

// VideoAnalysisResult is the output of the video analysis pipeline.
type VideoAnalysisResult struct {
	DurationSeconds   float64            `json:"duration_seconds"`
	FramesAnalyzed    int                `json:"frames_analyzed"`
	SceneDescriptions []SceneDescription `json:"scene_descriptions"`
	KeyTopics         []string           `json:"key_topics"`
	Transcript        string             `json:"transcript"`
	Confidence        float64            `json:"confidence"`
}

// VideoRequest holds the parameters for video analysis.
type VideoRequest struct {
	VideoBytes  []byte
	MIMEType    string
	FPS         float64 // default 1.0
	MaxFrames   int     // default 200
	WorkspaceID string
}

// VideoProcessor runs the full video analysis pipeline:
// 1. Extract frames via ffmpeg
// 2. Extract audio track
// 3. Transcribe audio via STT
// 4. Analyze frames in batches via vision
// 5. Merge scenes + transcript → extract topics
type VideoProcessor struct {
	ffmpeg    *connectors.FFmpegClient
	sttRouter *gateway.STTRouter
	vision    *VisionProcessor
}

// NewVideoProcessor creates a video processor.
func NewVideoProcessor(
	ffmpeg *connectors.FFmpegClient,
	sttRouter *gateway.STTRouter,
	vision *VisionProcessor,
) *VideoProcessor {
	return &VideoProcessor{
		ffmpeg:    ffmpeg,
		sttRouter: sttRouter,
		vision:    vision,
	}
}

// Analyze runs the full 5-step video analysis pipeline.
func (vp *VideoProcessor) Analyze(ctx context.Context, req VideoRequest) (*VideoAnalysisResult, error) {
	if req.FPS <= 0 {
		req.FPS = 1.0
	}
	if req.MaxFrames <= 0 {
		req.MaxFrames = 200
	}

	// Step 1: Extract frames via ffmpeg.
	frames, err := vp.ffmpeg.ExtractFrames(ctx, req.VideoBytes, req.FPS, req.MaxFrames)
	if err != nil {
		return nil, fmt.Errorf("video_processor: extract_frames: %w", err)
	}

	// Step 2: Extract audio track (nil if no audio).
	audioBytes, _ := vp.ffmpeg.ExtractAudio(ctx, req.VideoBytes)

	// Step 3: Transcribe audio via STT pipeline (non-fatal).
	transcript := ""
	if len(audioBytes) > 0 && vp.sttRouter != nil {
		// Write audio to temp file and get URL for STT.
		tmpPath, writeErr := connectors.WriteTempFileExported(audioBytes, "brevio-audio-*.mp3")
		if writeErr == nil {
			sttResult, sttErr := vp.sttRouter.TranscribeURL(ctx, "file://"+tmpPath, gateway.STTOptions{})
			if sttErr == nil && sttResult != nil {
				transcript = sttResult.Text
			}
		}
	}

	// Step 4: Analyze frames — build scene descriptions.
	scenes := vp.analyzeFrames(ctx, frames, req)

	// Step 5: Merge and produce final result.
	duration, _ := vp.ffmpeg.GetDurationSeconds(ctx, req.VideoBytes)

	confidence := 0.70
	if transcript != "" {
		confidence = 0.90
	}

	sceneTexts := make([]string, 0, len(scenes))
	for _, s := range scenes {
		sceneTexts = append(sceneTexts, s.Description)
	}

	return &VideoAnalysisResult{
		DurationSeconds:   duration,
		FramesAnalyzed:    len(scenes),
		SceneDescriptions: scenes,
		KeyTopics:         extractTopics(sceneTexts, transcript),
		Transcript:        transcript,
		Confidence:        confidence,
	}, nil
}

func (vp *VideoProcessor) analyzeFrames(ctx context.Context, frames [][]byte, req VideoRequest) []SceneDescription {
	if vp.vision == nil || len(frames) == 0 {
		return nil
	}

	var scenes []SceneDescription
	for i, frame := range frames {
		// Process each frame as a vision extraction.
		exReq := ExtractionRequest{
			WorkspaceID: req.WorkspaceID,
			TurnID:      fmt.Sprintf("video-frame-%d", i),
			Attachments: []ImageAttachment{{
				Data:     frame,
				MimeType: "image/jpeg",
			}},
		}
		result, err := vp.vision.Process(ctx, exReq)
		if err != nil || result == nil {
			continue
		}

		timestamp := float64(i) / req.FPS
		scenes = append(scenes, SceneDescription{
			FrameIndex:  i,
			Timestamp:   timestamp,
			Description: result.NormalizedText,
		})
	}
	return scenes
}

func extractTopics(sceneTexts []string, transcript string) []string {
	// Simple keyword extraction from scene descriptions and transcript.
	combined := strings.Join(sceneTexts, " ") + " " + transcript
	words := strings.Fields(combined)
	freq := map[string]int{}
	for _, w := range words {
		clean := strings.ToLower(strings.Trim(w, ".,;:!?\"'()"))
		if len(clean) > 4 { // skip short words
			freq[clean]++
		}
	}

	// Return top 5 by frequency.
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range freq {
		if v >= 2 { // must appear at least twice
			sorted = append(sorted, kv{k, v})
		}
	}
	// Simple insertion sort (small N).
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].v > sorted[j-1].v; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	topics := make([]string, 0, 5)
	for i, kv := range sorted {
		if i >= 5 {
			break
		}
		topics = append(topics, kv.k)
	}
	return topics
}
