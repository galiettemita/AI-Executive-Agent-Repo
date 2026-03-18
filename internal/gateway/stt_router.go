package gateway

import (
	"context"
	"time"
)

const (
	sttRouterTimeout         = 10 * time.Second
	lowConfidenceThreshold   = 0.70
)

// STTRouter enhances STTService with quality-based parallel transcription.
// If the primary provider's confidence is below the threshold, the fallback
// is also invoked and the higher-confidence result is selected.
type STTRouter struct {
	primary  STTProvider
	fallback STTProvider
}

// NewSTTRouter creates a router. Fallback may be nil.
func NewSTTRouter(primary, fallback STTProvider) *STTRouter {
	return &STTRouter{primary: primary, fallback: fallback}
}

// TranscribeURL routes transcription with quality-based fallback.
// Falls back on primary failure or low confidence.
func (r *STTRouter) TranscribeURL(ctx context.Context, audioURL string, opts STTOptions) (*TranscriptResult, error) {
	// Attempt primary with timeout.
	primaryCtx, primaryCancel := context.WithTimeout(ctx, sttRouterTimeout)
	defer primaryCancel()

	primaryResult, err := r.primary.Transcribe(primaryCtx, audioURL, opts)

	// Fallback on primary failure.
	if err != nil {
		if r.fallback != nil {
			return r.fallback.Transcribe(ctx, audioURL, opts)
		}
		return nil, err
	}

	// Quality gate: if primary confidence < 0.70, run parallel fallback.
	if ConfidenceIsKnown(primaryResult.Confidence) &&
		primaryResult.Confidence < lowConfidenceThreshold &&
		r.fallback != nil {
		fallbackResult, fallbackErr := r.fallback.Transcribe(ctx, audioURL, opts)
		if fallbackErr == nil {
			return selectBestTranscript(primaryResult, fallbackResult), nil
		}
	}

	return primaryResult, nil
}

// selectBestTranscript returns the higher-confidence result.
func selectBestTranscript(a, b *TranscriptResult) *TranscriptResult {
	if !ConfidenceIsKnown(a.Confidence) {
		return b
	}
	if !ConfidenceIsKnown(b.Confidence) {
		return a
	}
	if b.Confidence > a.Confidence {
		return b
	}
	return a
}
