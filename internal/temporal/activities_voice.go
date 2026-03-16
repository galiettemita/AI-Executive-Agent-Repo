package temporal

import (
	"github.com/brevio/brevio/internal/voice/worker"
)

// tokenSignerAdapter wraps LiveKitTokenSigner to match the call site in
// InitVoiceSessionActivity. Uses the official LiveKit SDK for token generation.
type tokenSignerAdapter struct {
	signer *worker.LiveKitTokenSigner
}

func newTokenSignerAdapter(apiKey, apiSecret string) (*tokenSignerAdapter, error) {
	s, err := worker.NewLiveKitTokenSigner(apiKey, apiSecret)
	if err != nil {
		return nil, err
	}
	return &tokenSignerAdapter{signer: s}, nil
}

func (t *tokenSignerAdapter) Sign(sessionID, workspaceID, roomName string) (string, error) {
	return t.signer.SignAgentToken(sessionID, workspaceID, roomName)
}
